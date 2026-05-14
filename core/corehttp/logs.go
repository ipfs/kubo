package corehttp

import (
	"bufio"
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
			done := make(chan struct{})

			// Close the pipe reader if the request context is canceled. This
			// is necessary to avoiding blocking on reading from the pipe
			// reader when the client terminates the request.
			go func() {
				select {
				case <-r.Context().Done(): // Client canceled request
				case <-n.Context().Done(): // Node shutdown
				case <-done: // log reader goroutine exitex
				}
				pipeReader.Close()
			}()

			errs := make(chan error, 1)

			go func() {
				defer close(errs)
				defer close(done)

				rdr := bufio.NewReader(pipeReader)
				for {
					// Read a line of log data and send it to the client.
					line, err := rdr.ReadString('\n')
					if err != nil {
						errs <- fmt.Errorf("error reading log message: %s", err)
						return
					}
					_, err = w.Write([]byte(line))
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
			log.Info("log API client connected")
			err := <-errs
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
		return mux, nil
	}
}
