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
	args := make([]string, 0)

	if cmd, err := commands.Root.Get(path[:len(path)-1]); err != nil {
		// 404 if there is no command at that path
		return nil, ErrNotFound

	} else if cmd.Subcommand(path[len(path)-1]) == nil {
		// if the last string in the path isn't a subcommand, use it as an argument
		// e.g. /objects/Qabc12345 (we are passing "Qabc12345" to the "objects" command)
		args = append(args, path[len(path)-1])
		path = path[:len(path)-1]
	}

	opts, args2 := parseOptions(r)
	args = append(args, args2...)

	// TODO: input stream (from request body)

	return cmds.NewRequest(path, opts, args, nil), nil
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
