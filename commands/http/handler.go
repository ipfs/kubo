package http

import (
	"errors"
	"io"
	"net/http"

	cmds "github.com/jbenet/go-ipfs/commands"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("commands/http")

type Handler struct {
	ctx    cmds.Context
	root   *cmds.Command
	origin string
}

var ErrNotFound = errors.New("404 page not found")

const (
	streamHeader           = "X-Stream-Output"
	channelHeader          = "X-Chunked-Output"
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	transferEncodingHeader = "Transfer-Encoding"
)

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

func NewHandler(ctx cmds.Context, root *cmds.Command, origin string) *Handler {
	// allow whitelisted origins (so we can make API requests from the browser)
	if len(origin) > 0 {
		log.Info("Allowing API requests from origin: " + origin)
	}

	return &Handler{ctx, root, origin}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Incoming API request: ", r.URL)

	if len(i.origin) > 0 {
		w.Header().Set("Access-Control-Allow-Origin", i.origin)
	}
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	req, err := Parse(r, i.root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}
	req.SetContext(i.ctx)

	// call the command
	res := i.root.Call(req)

	// set the Content-Type based on res output
	if _, ok := res.Output().(io.Reader); ok {
		// we don't set the Content-Type for streams, so that browsers can MIME-sniff the type themselves
		// we set this header so clients have a way to know this is an output stream
		// (not marshalled command output)
		// TODO: set a specific Content-Type if the command response needs it to be a certain type
		w.Header().Set(streamHeader, "1")

	} else {
		enc, found, err := req.Option(cmds.EncShort).String()
		if err != nil || !found {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mime := mimeTypes[enc]
		w.Header().Set(contentTypeHeader, mime)
	}

	// if the res output is a channel, set a custom header for it
	if _, ok := res.Output().(chan interface{}); ok {
		w.Header().Set(channelHeader, "1")
	}

	// if response contains an error, write an HTTP error status code
	if e := res.Error(); e != nil {
		if e.Code == cmds.ErrClient {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	out, err := res.Reader()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set(contentTypeHeader, "text/plain")
		w.Write([]byte(err.Error()))
		return
	}

	err = copyChunks(w, out)
	if err != nil {
		log.Error(err)
	}
}

// Copies from an io.Reader to a http.ResponseWriter.
// Flushes chunks over HTTP stream as they are read (if supported by transport).
func copyChunks(w http.ResponseWriter, out io.Reader) error {
	buf := make([]byte, 32*1024)

	for {
		n, err := out.Read(buf)

		if n > 0 {
			_, err := w.Write(buf[0:n])
			if err != nil {
				return err
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		if err != nil {
			return err
		}
	}
	return nil
}
