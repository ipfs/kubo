package missinggo

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// A http.ResponseWriter that tracks the status of the response. The status
// code, and number of bytes written for example.
type StatusResponseWriter struct {
	RW           http.ResponseWriter
	Code         int
	BytesWritten int64
}

var _ http.ResponseWriter = &StatusResponseWriter{}

func (me *StatusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return me.RW.(http.Hijacker).Hijack()
}

func (me *StatusResponseWriter) CloseNotify() <-chan bool {
	return me.RW.(http.CloseNotifier).CloseNotify()
}

func (me *StatusResponseWriter) Flush() {
	me.RW.(http.Flusher).Flush()
}

func (me *StatusResponseWriter) Header() http.Header {
	return me.RW.Header()
}

func (me *StatusResponseWriter) Write(b []byte) (n int, err error) {
	if me.Code == 0 {
		me.Code = 200
	}
	n, err = me.RW.Write(b)
	me.BytesWritten += int64(n)
	return
}

func (me *StatusResponseWriter) WriteHeader(code int) {
	me.RW.WriteHeader(code)
	me.Code = code
}

type ReaderFromStatusResponseWriter struct {
	StatusResponseWriter
	io.ReaderFrom
}

func NewReaderFromStatusResponseWriter(w http.ResponseWriter) *ReaderFromStatusResponseWriter {
	return &ReaderFromStatusResponseWriter{
		StatusResponseWriter{RW: w},
		w.(io.ReaderFrom),
	}
}
