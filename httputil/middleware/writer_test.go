package middleware

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStatusWriter() (*statusWriter, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	return &statusWriter{ResponseWriter: rec}, rec
}

func TestStatusWriter_WriteHeader_Single(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	sw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, sw.Status())
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestStatusWriter_WriteHeader_Idempotent(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	sw.WriteHeader(http.StatusNotFound)
	sw.WriteHeader(http.StatusOK)
	assert.Equal(t, http.StatusNotFound, sw.Status())
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestStatusWriter_Status_DefaultOK(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	assert.Equal(t, http.StatusOK, sw.Status())
}

func TestStatusWriter_Write_ImplicitHeader(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	n, err := sw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, http.StatusOK, sw.Status())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "hello", rec.Body.String())
}

func TestStatusWriter_Write_ExplicitHeaderThenWrite(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	sw.WriteHeader(http.StatusAccepted)
	n, err := sw.Write([]byte("data"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, http.StatusAccepted, sw.Status())
	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Equal(t, "data", rec.Body.String())
}

func TestStatusWriter_BytesWritten_Multiple(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	_, _ = sw.Write([]byte("abc"))
	_, _ = sw.Write([]byte("defgh"))
	assert.Equal(t, 8, sw.BytesWritten())
}

func TestStatusWriter_BytesWritten_Zero(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	assert.Equal(t, 0, sw.BytesWritten())
}

func TestStatusWriter_Flush_Supported(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}
	sw.Flush()
	assert.True(t, rec.Flushed)
}

type noFlushWriter struct {
	http.ResponseWriter
}

func TestStatusWriter_Flush_NotSupported(t *testing.T) {
	t.Parallel()
	sw := &statusWriter{ResponseWriter: &noFlushWriter{}}
	sw.Flush()
}

type fakeHijacker struct {
	http.ResponseWriter
	conn net.Conn
}

func (f *fakeHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	rw := bufio.NewReadWriter(bufio.NewReader(f.conn), bufio.NewWriter(f.conn))
	return f.conn, rw, nil
}

func TestStatusWriter_Hijack_Supported(t *testing.T) {
	t.Parallel()
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	h := &fakeHijacker{ResponseWriter: httptest.NewRecorder(), conn: server}
	sw := &statusWriter{ResponseWriter: h}
	conn, rw, err := sw.Hijack()
	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.NotNil(t, rw)
}

func TestStatusWriter_Hijack_NotSupported(t *testing.T) {
	t.Parallel()
	sw := &statusWriter{ResponseWriter: httptest.NewRecorder()}
	conn, rw, err := sw.Hijack()
	assert.Nil(t, conn)
	assert.Nil(t, rw)
	require.Error(t, err)
	assert.Equal(t, "hijack not supported", err.Error())
}

func TestStatusWriter_Unwrap(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}
	assert.Equal(t, rec, sw.Unwrap())
}

func TestStatusWriter_ReadFrom_WithReaderFrom(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	src := strings.NewReader("readfrom data")
	n, err := sw.ReadFrom(src)
	require.NoError(t, err)
	assert.Equal(t, int64(13), n)
	assert.Equal(t, 13, sw.BytesWritten())
	assert.Equal(t, http.StatusOK, sw.Status())
	assert.Equal(t, "readfrom data", rec.Body.String())
}

type plainWriter struct {
	buf bytes.Buffer
	hdr http.Header
}

func (w *plainWriter) Header() http.Header         { return w.hdr }
func (w *plainWriter) Write(b []byte) (int, error) { return w.buf.Write(b) }
func (w *plainWriter) WriteHeader(_ int)           {}

func TestStatusWriter_ReadFrom_FallbackIOCopy(t *testing.T) {
	t.Parallel()
	pw := &plainWriter{hdr: make(http.Header)}
	sw := &statusWriter{ResponseWriter: pw}
	src := strings.NewReader("fallback data")
	n, err := sw.ReadFrom(src)
	require.NoError(t, err)
	assert.Equal(t, int64(13), n)
	assert.Equal(t, 13, sw.BytesWritten())
	assert.Equal(t, "fallback data", pw.buf.String())
}

func TestStatusWriter_ReadFrom_ExplicitStatus(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	sw.WriteHeader(http.StatusCreated)
	src := strings.NewReader("body")
	n, err := sw.ReadFrom(src)
	require.NoError(t, err)
	assert.Equal(t, int64(4), n)
	assert.Equal(t, http.StatusCreated, sw.Status())
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestStatusWriter_ClaimHeaderSent_FirstTrue(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	assert.True(t, sw.claimHeaderSent())
}

func TestStatusWriter_ClaimHeaderSent_SecondFalse(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	sw.claimHeaderSent()
	assert.False(t, sw.claimHeaderSent())
}

func TestStatusWriter_ClaimHeaderSent_AfterWrite(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	_, _ = sw.Write([]byte("x"))
	assert.False(t, sw.claimHeaderSent())
}

func TestStatusWriter_ClaimHeaderSent_AfterWriteHeader(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	sw.WriteHeader(http.StatusOK)
	assert.False(t, sw.claimHeaderSent())
}

func TestStatusWriter_Write_PreservesExplicitStatus(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	sw.WriteHeader(http.StatusBadRequest)
	_, _ = sw.Write([]byte("err"))
	assert.Equal(t, http.StatusBadRequest, sw.Status())
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

type errorWriter struct {
	hdr http.Header
}

func (w *errorWriter) Header() http.Header         { return w.hdr }
func (w *errorWriter) Write(_ []byte) (int, error) { return 0, errors.New("write failed") }
func (w *errorWriter) WriteHeader(_ int)           {}

func TestStatusWriter_Write_Error(t *testing.T) {
	t.Parallel()
	ew := &errorWriter{hdr: make(http.Header)}
	sw := &statusWriter{ResponseWriter: ew}
	n, err := sw.Write([]byte("data"))
	assert.Equal(t, 0, n)
	require.Error(t, err)
	assert.Equal(t, 0, sw.BytesWritten())
}

func TestStatusWriter_ReadFrom_Error(t *testing.T) {
	t.Parallel()
	ew := &errorWriter{hdr: make(http.Header)}
	sw := &statusWriter{ResponseWriter: ew}
	n, err := sw.ReadFrom(strings.NewReader("data"))
	assert.Equal(t, int64(0), n)
	assert.Error(t, err)
}

func TestStatusWriter_Concurrent_WriteHeader(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(code int) {
			defer wg.Done()
			sw.WriteHeader(code)
		}(200 + i)
	}
	wg.Wait()
	status := sw.Status()
	assert.GreaterOrEqual(t, status, 200)
	assert.Less(t, status, 250)
}

func TestStatusWriter_Concurrent_Write(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, _ = sw.Write([]byte("ab"))
		})
	}
	wg.Wait()
	assert.Equal(t, 100, sw.BytesWritten())
	assert.Equal(t, http.StatusOK, sw.Status())
}

func TestStatusWriter_Concurrent_MixedOps(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		sw.WriteHeader(http.StatusCreated)
	}()
	go func() {
		defer wg.Done()
		_, _ = sw.Write([]byte("data"))
	}()
	go func() {
		defer wg.Done()
		_ = sw.Status()
		_ = sw.BytesWritten()
	}()
	wg.Wait()
	assert.GreaterOrEqual(t, sw.BytesWritten(), 0)
}

func TestStatusWriter_LargeWrite(t *testing.T) {
	t.Parallel()
	sw, rec := newStatusWriter()
	data := make([]byte, 1<<16)
	for i := range data {
		data[i] = 'x'
	}
	n, err := sw.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, len(data), sw.BytesWritten())
	assert.Equal(t, len(data), rec.Body.Len())
}

func TestStatusWriter_ReadFrom_Empty(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	n, err := sw.ReadFrom(strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	assert.Equal(t, 0, sw.BytesWritten())
}

func TestStatusWriter_Write_Empty(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	n, err := sw.Write(nil)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 0, sw.BytesWritten())
}

func TestStatusWriter_ImplementsInterfaces(t *testing.T) {
	t.Parallel()
	sw, _ := newStatusWriter()
	assert.Implements(t, (*http.ResponseWriter)(nil), sw)
	assert.Implements(t, (*http.Flusher)(nil), sw)
	assert.Implements(t, (*http.Hijacker)(nil), sw)
	assert.Implements(t, (*io.ReaderFrom)(nil), sw)
}
