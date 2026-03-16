package httputil

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

// ErrSSEClosed is returned by Send and SendJSON after the writer has been closed.
var ErrSSEClosed = errors.New("SSE writer closed")

// ErrSSEPayloadTooLarge is returned by Send and SendJSON when the event payload would exceed MaxEventBytes.
var ErrSSEPayloadTooLarge = errors.New("SSE event payload exceeds size limit")

// SSEWriter sends Server-Sent Events. Create with NewSSEWriter or NewSSEWriterWithLimit.
// Send and SendJSON are safe for concurrent use. Call Close when done.
// The 200 response is committed on the first successful Send or SendJSON, not in NewSSEWriter.
type SSEWriter struct {
	w             http.ResponseWriter
	flusher       http.Flusher
	mu            sync.Mutex
	done          atomic.Bool
	headerSent    bool
	maxEventBytes int64
}

// SSEOption configures NewSSEWriterWithLimit.
type SSEOption func(*SSEWriter)

// defaultSSEMaxEventBytes is the default maximum size of a single event (event + data lines).
const defaultSSEMaxEventBytes = 64 * 1024

// MaxEventBytes limits the size of a single event (event + data lines); 0 means use default (64KB).
func MaxEventBytes(n int64) SSEOption {
	return func(s *SSEWriter) { s.maxEventBytes = n }
}

// NewSSEWriter sets SSE headers on w and returns an SSEWriter. The 200 response is sent on the first Send or SendJSON. The second return value is false if w does not implement http.Flusher.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, bool) {
	return NewSSEWriterWithLimit(w)
}

// NewSSEWriterWithLimit is like NewSSEWriter but accepts options (e.g. MaxEventBytes). Does not write 200 until the first Send or SendJSON. If MaxEventBytes is not set, a default of 64KB is used to limit memory for untrusted event sizes.
func NewSSEWriterWithLimit(w http.ResponseWriter, opts ...SSEOption) (*SSEWriter, bool) {
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	s := &SSEWriter{w: w, flusher: f, maxEventBytes: defaultSSEMaxEventBytes}
	for _, opt := range opts {
		opt(s)
	}
	if s.maxEventBytes == 0 {
		s.maxEventBytes = defaultSSEMaxEventBytes
	}
	return s, true
}

func sanitizeSSEField(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// Send writes one SSE message (optional event line, then data lines, then newline) and flushes. Event and data are sanitized (newlines in event replaced). Returns ErrSSEPayloadTooLarge if the message would exceed MaxEventBytes.
func (s *SSEWriter) Send(event, data string) error {
	if s.done.Load() {
		return ErrSSEClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done.Load() {
		return ErrSSEClosed
	}
	if !s.headerSent {
		s.w.WriteHeader(http.StatusOK)
		s.flusher.Flush()
		s.headerSent = true
	}
	var eventPart string
	if event != "" {
		event = sanitizeSSEField(event)
		eventPart = "event: " + event + "\n"
	}
	data = strings.ReplaceAll(data, "\r\n", "\n")
	data = strings.ReplaceAll(data, "\r", "\n")
	var dataPart int64
	for _, line := range strings.Split(data, "\n") {
		dataPart += int64(len("data: ") + len(line) + 1)
	}
	total := int64(len(eventPart)) + dataPart + 1
	if s.maxEventBytes > 0 && total > s.maxEventBytes {
		return ErrSSEPayloadTooLarge
	}
	if eventPart != "" {
		_, err := s.w.Write([]byte(eventPart))
		if err != nil {
			return err
		}
	}
	for _, line := range strings.Split(data, "\n") {
		_, err := s.w.Write([]byte("data: " + line + "\n"))
		if err != nil {
			return err
		}
	}
	_, err := s.w.Write([]byte("\n"))
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendJSON marshals v to JSON and sends it as the data payload using Send.
func (s *SSEWriter) SendJSON(event string, v any) error {
	if s.done.Load() {
		return ErrSSEClosed
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Send(event, string(raw))
}

// Close marks the writer as done; subsequent Send or SendJSON calls return ErrSSEClosed.
func (s *SSEWriter) Close() {
	s.mu.Lock()
	s.done.Store(true)
	s.mu.Unlock()
}
