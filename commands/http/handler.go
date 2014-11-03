package http

import (
	"errors"
	"io"
	"net/http"

	cmds "github.com/jbenet/go-ipfs/commands"
)

type Handler struct {
	Ctx  cmds.Context
	Root *cmds.Command
}

var ErrNotFound = errors.New("404 page not found")

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := Parse(r, i.Root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}
	req.SetContext(i.Ctx)

	// call the command
	res := i.Root.Call(req)

	// set the Content-Type based on res output
	if _, ok := res.Output().(io.Reader); ok {
		// TODO: set based on actual Content-Type of file
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		enc, _ := req.Option(cmds.EncShort)
		mime := mimeTypes[enc.(string)]
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

	_, err = io.Copy(w, res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
	}
}
