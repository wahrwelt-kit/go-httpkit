package httputil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

const maxContentDispositionFilenameLen = 255

// ErrStreamTruncated is returned by RenderStreamLimited when the source exceeds maxBytes and the response was truncated
var ErrStreamTruncated = errors.New("response stream truncated: source exceeded max bytes")

// ErrInvalidContentType is returned when contentType contains CR/LF or disallowed characters (header injection)
var ErrInvalidContentType = errors.New("content-type contains invalid characters")

// sanitizeContentType sanitizes a content type for use in a Content-Type header
func sanitizeContentType(contentType string) (string, error) {
	for _, r := range contentType {
		if r == '\r' || r == '\n' || r < 0x20 || r == 0x7F {
			return "", ErrInvalidContentType
		}
		if r > 0x7E {
			return "", ErrInvalidContentType
		}
	}
	return strings.TrimSpace(contentType), nil
}

// sanitizeContentDispositionFilename sanitizes a filename for use in a Content-Disposition header
func sanitizeContentDispositionFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		if r < 32 || r == 127 || r == '"' || r == '\\' {
			continue
		}
		b.WriteRune(r)
		if b.Len() >= maxContentDispositionFilenameLen {
			break
		}
	}
	s := b.String()
	if s == "" || s == "." {
		return "download"
	}
	return s
}

// RenderJSONAttachment encodes data as JSON and writes it with Content-Disposition attachment and sanitized filename
func RenderJSONAttachment[T any](w http.ResponseWriter, data T, filename string) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		return fmt.Errorf("encode json attachment: %w", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{paramFilename: sanitizeContentDispositionFilename(filename)}))
	_, err := w.Write(buf.Bytes())
	return err
}

// RenderStream streams the response with Content-Disposition attachment. Caller is responsible for closing rc after the function returns (e.g. if rc implements io.Closer, call rc.Close() in defer)
// For untrusted or unbounded sources use RenderStreamLimited to cap the response size
// contentType is sent as-is in the Content-Type header; it MUST NOT contain user-controlled input-validate or use a fixed allowlist to avoid header injection (e.g. XSS via attachment)
func RenderStream(w http.ResponseWriter, contentType, filename string, rc io.Reader) error {
	return RenderStreamLimited(w, contentType, filename, rc, 0)
}

// RenderStreamLimited is like RenderStream but limits the number of bytes copied from rc to maxBytes
// If maxBytes <= 0, no limit is applied. When maxBytes > 0 and the source exceeds the limit, the response
// is already committed (headers and up to maxBytes sent) and the function returns ErrStreamTruncated so the
// caller can log or handle the truncation. Use a pre-buffered reader or Content-Length if you need to reject before sending
func RenderStreamLimited(w http.ResponseWriter, contentType, filename string, rc io.Reader, maxBytes int64) error {
	if rc == nil {
		return errors.New("reader is nil")
	}
	ct, err := sanitizeContentType(contentType)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{paramFilename: sanitizeContentDispositionFilename(filename)}))
	reader := rc
	if maxBytes > 0 {
		reader = io.LimitReader(rc, maxBytes)
	}
	_, err = io.Copy(w, reader)
	if err != nil {
		return err
	}
	if maxBytes > 0 {
		probe := make([]byte, 1)
		n, err := rc.Read(probe)
		if n > 0 {
			return ErrStreamTruncated
		}
		if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}

// RenderBytes writes raw bytes with Content-Type and Content-Disposition attachment (filename sanitized)
func RenderBytes(w http.ResponseWriter, contentType, filename string, data []byte) error {
	ct, err := sanitizeContentType(contentType)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{paramFilename: sanitizeContentDispositionFilename(filename)}))
	_, err = w.Write(data)
	return err
}
