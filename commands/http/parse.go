package http

import (
	"errors"
	"net/http"
	"strings"

	cmds "github.com/jbenet/go-ipfs/commands"
)

// Parse parses the data in a http.Request and returns a command Request object
func Parse(r *http.Request, root *cmds.Command) (cmds.Request, error) {
	if !strings.HasPrefix(r.URL.Path, ApiPath) {
		return nil, errors.New("Unexpected path prefix")
	}
	path := strings.Split(strings.TrimPrefix(r.URL.Path, ApiPath+"/"), "/")

	stringArgs := make([]string, 0)

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
		stringArgs = append(stringArgs, path[len(path)-1])
		path = path[:len(path)-1]

	} else {
		cmd = sub
	}

	opts, stringArgs2 := parseOptions(r)
	stringArgs = append(stringArgs, stringArgs2...)

	args := make([]interface{}, 0)

	// count required argument definitions
	lenRequired := 0
	for _, argDef := range cmd.Arguments {
		if argDef.Required {
			lenRequired++
		}
	}

	// count the number of provided argument values
	valCount := len(stringArgs)
	// TODO: add total number of parts in request body (instead of just 1 if body is present)
	if r.Body != nil {
		valCount += 1
	}

	for _, argDef := range cmd.Arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if valCount <= lenRequired && !argDef.Required {
			continue
		}
		valCount--

		if argDef.Type == cmds.ArgString {
			if argDef.Variadic {
				for _, s := range stringArgs {
					args = append(args, s)
				}

			} else if len(stringArgs) > 0 {
				args = append(args, stringArgs[0])
				stringArgs = stringArgs[1:]

			} else {
				break
			}

		} else {
			// TODO: create multipart streams for file args
			args = append(args, r.Body)
		}
	}

	if valCount-1 > 0 {
		args = append(args, make([]interface{}, valCount-1))
	}

	req := cmds.NewRequest(path, opts, args, cmd)

	err = cmd.CheckArguments(req)
	if err != nil {
		return nil, err
	}

	return req, nil
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
