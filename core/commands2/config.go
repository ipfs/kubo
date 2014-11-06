package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
)

type ConfigField struct {
	Key   string
	Value interface{}
}

var configCmd = &cmds.Command{
	Arguments: []cmds.Argument{
		cmds.Argument{"key", cmds.ArgString, true, false},
		cmds.Argument{"value", cmds.ArgString, false, false},
	},
	Help: `ipfs config <key> [value] - Get/Set ipfs config values.

    ipfs config <key>          - Get value of <key>
    ipfs config <key> <value>  - Set value of <key> to <value>
    ipfs config show           - Show config file
    ipfs config edit           - Edit config file in $EDITOR

Examples:

  Get the value of the 'datastore.path' key:

      ipfs config datastore.path

  Set the value of the 'datastore.path' key:

      ipfs config datastore.path ~/.go-ipfs/datastore

`,
	Run: func(res cmds.Response, req cmds.Request) {
		args := req.Arguments()

		key, ok := args[0].(string)
		if !ok {
			res.SetError(errors.New("cast error"), cmds.ErrNormal)
			return
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var value string
		if len(args) == 2 {
			var ok bool
			value, ok = args[1].(string)
			if !ok {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}

			field, err := setConfig(filename, key, value)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			res.SetOutput(field)
			return

		} else {
			field, err := getConfig(filename, key)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			res.SetOutput(field)
			return
		}
	},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.Text: func(res cmds.Response) ([]byte, error) {
			v := res.Output().(*ConfigField)

			s := ""
			if len(res.Request().Arguments()) == 2 {
				s += fmt.Sprintf("'%s' set to: ", v.Key)
			}

			marshalled, err := json.Marshal(v.Value)
			if err != nil {
				return nil, err
			}
			s += fmt.Sprintf("%s\n", marshalled)

			return []byte(s), nil
		},
	},
	Type: &ConfigField{},
	Subcommands: map[string]*cmds.Command{
		"show": configShowCmd,
		"edit": configEditCmd,
	},
}

var configShowCmd = &cmds.Command{
	Run: func(res cmds.Response, req cmds.Request) {
		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		reader, err := showConfig(filename)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(reader)
	},
}

var configEditCmd = &cmds.Command{
	Run: func(res cmds.Response, req cmds.Request) {
		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = editConfig(filename)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

func getConfig(filename string, key string) (*ConfigField, error) {
	value, err := config.ReadConfigKey(filename, key)
	if err != nil {
		return nil, fmt.Errorf("Failed to get config value: %s", err)
	}

	return &ConfigField{
		Key:   key,
		Value: value,
	}, nil
}

func setConfig(filename string, key, value string) (*ConfigField, error) {
	err := config.WriteConfigKey(filename, key, value)
	if err != nil {
		return nil, fmt.Errorf("Failed to set config value: %s", err)
	}

	return getConfig(filename, key)
}

func showConfig(filename string) (io.Reader, error) {
	// MAYBE_TODO: maybe we should omit privkey so we don't accidentally leak it?

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	//defer file.Close()

	return file, nil
}

func editConfig(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("ENV variable $EDITOR not set")
	}

	cmd := exec.Command("sh", "-c", editor+" "+filename)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
