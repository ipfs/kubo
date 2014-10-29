package http

import (
	"net/http"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core/commands"
)

// Parse parses the data in a http.Request and returns a command Request object
func Parse(r *http.Request) (cmds.Request, error) {
	path := strings.Split(r.URL.Path, "/")[3:]

	// 404 if there is no command at that path
	if _, err := commands.Root.Get(path); err != nil {
		return nil, ErrNotFound
	}

	opts, args := parseOptions(r)

	// TODO: input stream (from request body)

	return cmds.NewRequest(path, opts, args, nil), nil
}

func parseOptions(r *http.Request) (map[string]interface{}, []string) {
	opts := make(map[string]interface{})

	query := r.URL.Query()
	for k, v := range query {
		opts[k] = v[0]
	}

	// TODO: get more options from request body (formdata, json, etc)

	// default to setting encoding to JSON
	_, short := opts[cmds.EncShort]
	_, long := opts[cmds.EncLong]
	if !short && !long {
		opts[cmds.EncShort] = cmds.JSON
	}

	return opts, nil
}
