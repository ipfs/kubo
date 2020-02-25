package corehttp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"

	core "github.com/ipfs/go-ipfs/core"
)

func newProxyListener(conn net.Conn, laddr net.Addr) *proxyListener {
	pl := &proxyListener{
		conn: make(chan net.Conn, 1),
		addr: laddr,
	}
	pl.conn <- conn
	return pl
}

type proxyListener struct {
	conn chan net.Conn
	addr net.Addr
}

func (pl *proxyListener) Accept() (net.Conn, error) {
	c, ok := <-pl.conn
	if !ok {
		return nil, io.EOF
	}
	return c, nil
}

func (pl *proxyListener) Addr() net.Addr {
	return pl.addr
}

func (pl *proxyListener) Close() error {
	if c, err := pl.Accept(); err == nil {
		c.Close()
	}
	return nil
}

type proxyConn struct {
	*bufio.ReadWriter
	net.Conn
}

// ProxyOption transparently unwraps all inbound CONNECT requests.
func ProxyOption() ServeOption {
	return func(n *core.IpfsNode, l net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		childMux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// TODO remove below dead code?
			// Rationale Go does not support requests with CONNECT method :-(
			// https://golang.org/src/net/http/request.go#L111
			// Below code block never gets executed
			if r.Method == http.MethodConnect {
				hijacker, ok := w.(http.Hijacker)
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprint(w, "unable to proxy request")
					return
				}

				// The client may not write the request till we
				// send a response.
				w.WriteHeader(http.StatusOK)
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}

				conn, rw, err := hijacker.Hijack()
				if err != nil {
					// nothing we can do.
					return
				}
				// Serve always returns a non-nil error
				// nolint:errcheck
				http.Serve(
					newProxyListener(proxyConn{rw, conn}, l.Addr()),
					childMux,
				)
				return
			}
			childMux.ServeHTTP(w, r)
		})
		return childMux, nil
	}
}
