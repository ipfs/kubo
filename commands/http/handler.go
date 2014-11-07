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
	ctx  cmds.Context
	root *cmds.Command
}

var ErrNotFound = errors.New("404 page not found")

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

func NewHandler(ctx cmds.Context, root *cmds.Command) *Handler {
	return &Handler{ctx, root}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Incoming API request: ", r.URL)

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
		// TODO: set based on actual Content-Type of file
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		enc, _ := req.Option(cmds.EncShort)
		encStr, ok := enc.(string)
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mime := mimeTypes[encStr]
		w.Header().Set("Content-Type", mime)
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
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}

	io.Copy(w, out)
}
