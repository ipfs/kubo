package corehttp

import (
	"bytes"
	"io"
	"net"
	"net/http"

	core "github.com/ipfs/go-ipfs/core"

	logging "github.com/ipfs/go-log/v2"
)

type writeErrNotifier struct {
	w    io.Writer
	errs chan error
}

func newWriteErrNotifier(w io.Writer) (io.WriteCloser, <-chan error) {
	ch := make(chan error, 1)
	return &writeErrNotifier{
		w:    w,
		errs: ch,
	}, ch
}

func (w *writeErrNotifier) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	if err != nil {
		select {
		case w.errs <- err:
		default:
		}
	}
	if f, ok := w.w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

func (w *writeErrNotifier) Close() error {
	select {
	case w.errs <- io.EOF:
	default:
	}
	return nil
}

func LogOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			wnf, errs := newWriteErrNotifier(w)

			// FIXME(BLOCKING): This is a brittle solution and needs careful review.
			//  Ideally we should use an io.Pipe or similar, but in contrast
			//  with go-log@v1 where the driver was an io.Writer, here the log
			//  comes from an io.Reader, and we need to constantly read from it
			//  and then write to the HTTP response.
			pipeReader := logging.NewPipeReader()
			b := new(bytes.Buffer)
			go func() {
				for {
					// FIXME: We are not handling read errors.
					// FIXME: We may block on read and not catch the context
					//  cancellation.
					b.ReadFrom(pipeReader)
					b.WriteTo(wnf)
					select {
					case <-r.Context().Done():
						return
					default:
					}
				}
			}()

			// FIXME(BLOCKING): Verify this call replacing the `Event` API
			//  which has been removed in go-log v2.
			log.Debugf("log API client connected")
			<-errs
		})
		return mux, nil
	}
}
