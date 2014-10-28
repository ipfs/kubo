package http

import (
	"io"
	"net/http"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core/commands"
)

type Handler struct {
	Ctx cmds.Context
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Split(r.URL.Path, "/")[3:]
	opts := getOptions(r)

	// TODO: get args

	// ensure the requested command exists, otherwise 404
	_, err := commands.Root.Get(path)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 page not found"))
		return
	}

	// build the Request
	req := cmds.NewRequest(path, opts, nil, nil)
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

// getOptions returns the command options in the given HTTP request
// (from the querystring and request body)
func getOptions(r *http.Request) map[string]interface{} {
	opts := make(map[string]interface{})

	query := r.URL.Query()
	for k, v := range query {
		opts[k] = v[0]
	}

	// TODO: get more options from request body (formdata, json, etc)

	_, short := opts[cmds.EncShort]
	_, long := opts[cmds.EncLong]
	if !short && !long {
		opts[cmds.EncShort] = cmds.JSON
	}

	return opts
}
