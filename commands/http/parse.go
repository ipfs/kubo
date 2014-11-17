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

	// count required argument definitions
	numRequired := 0
	for _, argDef := range cmd.Arguments {
		if argDef.Required {
			numRequired++
		}
	}

	// count the number of provided argument values
	valCount := len(stringArgs)
	// TODO: add total number of parts in request body (instead of just 1 if body is present)
	if r.Body != nil && r.ContentLength != 0 {
		valCount += 1
	}

	args := make([]interface{}, valCount)

	valIndex := 0
	for _, argDef := range cmd.Arguments {
		// skip optional argument definitions if there aren't sufficient remaining values
		if valCount-valIndex <= numRequired && !argDef.Required {
			continue
		} else if argDef.Required {
			numRequired--
		}

		if argDef.Type == cmds.ArgString {
			if argDef.Variadic {
				for _, s := range stringArgs {
					args[valIndex] = s
					valIndex++
				}
				valCount -= len(stringArgs)

			} else if len(stringArgs) > 0 {
				args[valIndex] = stringArgs[0]
				stringArgs = stringArgs[1:]
				valIndex++

			} else {
				break
			}

		} else {
			// TODO: create multipart streams for file args
			args[valIndex] = r.Body
			valIndex++
		}
	}

	optDefs, err := root.GetOptions(path)
	if err != nil {
		return nil, err
	}

	req, err := cmds.NewRequest(path, opts, args, cmd, optDefs)
	if err != nil {
		return nil, err
	}

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
