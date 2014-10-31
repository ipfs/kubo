package http

import (
	"net/http"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// Parse parses the data in a http.Request and returns a command Request object
func Parse(r *http.Request, root *cmds.Command) (cmds.Request, error) {
	path := strings.Split(r.URL.Path, "/")[3:]
	args := make([]string, 0)

	cmd, err := root.Get(path[:len(path)-1])
	if err != nil {
		// 404 if there is no command at that path
		return nil, ErrNotFound

	} else if sub := cmd.Subcommand(path[len(path)-1]); sub == nil {
		if len(path) <= 1 {
			return nil, ErrNotFound
		}

		// if the last string in the path isn't a subcommand, use it as an argument
		// e.g. /objects/Qabc12345 (we are passing "Qabc12345" to the "objects" command)
		args = append(args, path[len(path)-1])
		path = path[:len(path)-1]

	} else {
		cmd = sub
	}

	if cmd.Private {
		return nil, ErrNotFound
	}

	opts, args2 := parseOptions(r)
	args = append(args, args2...)

	// TODO: make a way to send opts/args in request body
	//   (e.g. if form-data or form-urlencoded, then treat the same as querystring)
	// for now, to be simple, we just use the whole request body as the input stream
	// (r.Body will be nil if there is no request body, like in GET requests)
	in := r.Body

	return cmds.NewRequest(path, opts, args, in, cmd), nil
}

func parseOptions(r *http.Request) (map[string]interface{}, []string) {
	opts := make(map[string]interface{})
	var args []string

	query := r.URL.Query()
	for k, v := range query {
		if k == "arg" {
			args = v
		} else {
			opts[k] = v[0]
		}
	}

	// default to setting encoding to JSON
	_, short := opts[cmds.EncShort]
	_, long := opts[cmds.EncLong]
	if !short && !long {
		opts[cmds.EncShort] = cmds.JSON
	}

	return opts, args
}
