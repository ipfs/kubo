package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
)

type ConfigField struct {
	Key   string
	Value interface{}
}

var configCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "get and set IPFS config values",
		Synopsis: `
ipfs config <key>          - Get value of <key>
ipfs config <key> <value>  - Set value of <key> to <value>
ipfs config --show         - Show config file
ipfs config --edit         - Edit config file in $EDITOR
`,
		ShortDescription: `
ipfs config controls configuration variables. It works like 'git config'.
The configuration values are stored in a config file inside your IPFS
repository.`,
		LongDescription: `
ipfs config controls configuration variables. It works
much like 'git config'. The configuration values are stored in a config
file inside your IPFS repository.

EXAMPLES:

Get the value of the 'datastore.path' key:

  ipfs config datastore.path

Set the value of the 'datastore.path' key:

  ipfs config datastore.path ~/.go-ipfs/datastore
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key of the config entry (e.g. \"Addresses.API\")"),
		cmds.StringArg("value", false, false, "The value to set the config entry to"),
	},
	Options: []cmds.Option{
		cmds.StringOption("show", "s", "Show config file"),
		cmds.StringOption("edit", "e", "Edit config file in $EDITOR"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		args := req.Arguments()

		key, ok := args[0].(string)
		if !ok {
			return nil, u.ErrCast()
		}

		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			return nil, err
		}

		var value string
		if len(args) == 2 {
			var ok bool
			value, ok = args[1].(string)
			if !ok {
				return nil, u.ErrCast()
			}

			return setConfig(filename, key, value)

		} else {
			return getConfig(filename, key)
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
	Description: "Outputs the content of the config file",
	Help: `WARNING: Your private key is stored in the config file, and it will be
included in the output of this command.
`,

	Run: func(req cmds.Request) (interface{}, error) {
		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			return nil, err
		}

		return showConfig(filename)
	},
}

var configEditCmd = &cmds.Command{
	Description: "Opens the config file for editing in $EDITOR",
	Help: `To use 'ipfs config edit', you must have the $EDITOR environment
variable set to your preferred text editor.
`,

	Run: func(req cmds.Request) (interface{}, error) {
		filename, err := config.Filename(req.Context().ConfigRoot)
		if err != nil {
			return nil, err
		}

		return nil, editConfig(filename)
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
	// TODO maybe we should omit privkey so we don't accidentally leak it?

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
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
