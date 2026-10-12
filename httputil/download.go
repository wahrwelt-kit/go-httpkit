package httputil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const maxContentDispositionFilenameLen = 255

// ErrStreamTooLarge is returned by RenderStreamLimited when the source exceeds maxBytes before the response is written
var ErrStreamTooLarge = errors.New("response stream exceeds max bytes")

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
		runeLen := utf8.RuneLen(r)
		if runeLen < 0 {
			continue
		}
		if b.Len()+runeLen > maxContentDispositionFilenameLen {
			break
		}
		b.WriteRune(r)
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
	raw, err := marshalJSON(data)
	if err != nil {
		return fmt.Errorf("encode json attachment: %w", err)
	}
	if _, err := buf.Write(raw); err != nil {
		return fmt.Errorf("encode json attachment: %w", err)
	}
	if err := buf.WriteByte('\n'); err != nil {
		return fmt.Errorf("encode json attachment: %w", err)
	}
	writeAttachmentHeaders(w, "application/json; charset=utf-8", filename)
	_, err = w.Write(buf.Bytes())
	return err
}

// RenderStream streams the response with Content-Disposition attachment. Caller is responsible for closing rc after the function returns (e.g. if rc implements io.Closer, call rc.Close() in defer)
// Use RenderStreamLimited when the response must be rejected before write if it exceeds a maximum size
// contentType must be header-safe ASCII; dynamic values should still come from a service-owned allowlist
func RenderStream(w http.ResponseWriter, contentType, filename string, rc io.Reader) error {
	return RenderStreamLimited(w, contentType, filename, rc, 0)
}

// RenderStreamLimited is like RenderStream but limits the number of bytes copied from rc to maxBytes
// If maxBytes <= 0, no limit is applied and the response streams directly
// If maxBytes > 0, the source is buffered up to maxBytes and oversized sources return ErrStreamTooLarge before headers or body are written
func RenderStreamLimited(w http.ResponseWriter, contentType, filename string, rc io.Reader, maxBytes int64) error {
	if rc == nil {
		return errors.New("reader is nil")
	}
	ct, err := sanitizeContentType(contentType)
	if err != nil {
		return err
	}
	if maxBytes > 0 {
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, io.LimitReader(rc, maxBytes+1)); err != nil {
			return err
		}
		if int64(buf.Len()) > maxBytes {
			return ErrStreamTooLarge
		}
		writeAttachmentHeaders(w, ct, filename)
		_, err = w.Write(buf.Bytes())
		return err
	}
	writeAttachmentHeaders(w, ct, filename)
	_, err = io.Copy(w, rc)
	return err
}

// RenderBytes writes raw bytes with Content-Type and Content-Disposition attachment (filename sanitized)
func RenderBytes(w http.ResponseWriter, contentType, filename string, data []byte) error {
	ct, err := sanitizeContentType(contentType)
	if err != nil {
		return err
	}
	writeAttachmentHeaders(w, ct, filename)
	_, err = w.Write(data)
	return err
}

func writeAttachmentHeaders(w http.ResponseWriter, contentType, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{paramFilename: sanitizeContentDispositionFilename(filename)}))
}
