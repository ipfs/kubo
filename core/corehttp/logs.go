package corehttp

import (
	"bytes"
	"fmt"
	"net"
	"net/http"

	logging "github.com/ipfs/go-log/v2"
	core "github.com/ipfs/kubo/core"
)

func LogOption() ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
			// The log data comes from an io.Reader, and we need to constantly
			// read from it and then write to the HTTP response.
			pipeReader := logging.NewPipeReader()
			defer pipeReader.Close()

			errs := make(chan error, 1)

			go func() {
				defer close(errs)
				var b bytes.Buffer
				for {
					// FIXME: We may block on read and not catch the context
					// cancellation.
					_, err := b.ReadFrom(pipeReader)
					if err != nil {
						errs <- fmt.Errorf("error reading log event: %s", err)
						return
					}
					_, err = b.WriteTo(w)
					if err != nil {
						// Failed to write to client, probably disconnected.
						return
					}
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					if r.Context().Err() != nil {
						return
					}
				}
			}()
			err := <-errs
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
		return mux, nil
	}
}
