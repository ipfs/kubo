package http

import (
	"errors"
	"io"
	"net/http"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core/commands"
)

type Handler struct {
	Ctx cmds.Context
}

var ErrNotFound = errors.New("404 page not found")

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req, err := Parse(r)
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
	res := commands.Root.Call(req)

	// set the Content-Type based on res output
	if _, ok := res.Value().(io.Reader); ok {
		// TODO: set based on actual Content-Type of file
		w.Header().Set("Content-Type", "application/octet-stream")
	} else {
		// TODO: get proper MIME type for encoding from multicodec lib
		enc, _ := req.Option(cmds.EncShort)
		w.Header().Set("Content-Type", "application/"+enc.(string))
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
