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

	cmds "github.com/ipfs/go-ipfs/commands"
	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
)

type ConfigField struct {
	Key   string
	Value interface{}
}

var ConfigCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and set IPFS config values.",
		ShortDescription: `
'ipfs config' controls configuration variables. It works like 'git config'.
The configuration values are stored in a config file inside your IPFS
repository.`,
		LongDescription: `
'ipfs config' controls configuration variables. It works
much like 'git config'. The configuration values are stored in a config
file inside your IPFS repository.

Examples:

Get the value of the 'datastore.path' key:

  $ ipfs config datastore.path

Set the value of the 'datastore.path' key:

  $ ipfs config datastore.path ~/.ipfs/datastore
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key of the config entry (e.g. \"Addresses.API\")."),
		cmds.StringArg("value", false, false, "The value to set the config entry to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption("bool", "Set a boolean value. Default: false."),
		cmds.BoolOption("json", "Parse stringified JSON. Default: false."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		args := req.Arguments()
		key := args[0]

		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer r.Close()

		var output *ConfigField
		if len(args) == 2 {
			value := args[1]

			if parseJson, _, _ := req.Option("json").Bool(); parseJson {
				var jsonVal interface{}
				if err := json.Unmarshal([]byte(value), &jsonVal); err != nil {
					err = fmt.Errorf("failed to unmarshal json. %s", err)
					res.SetError(err, cmds.ErrNormal)
					return
				}

				output, err = setConfig(r, key, jsonVal)
			} else if isbool, _, _ := req.Option("bool").Bool(); isbool {
				output, err = setConfig(r, key, value == "true")
			} else {
				output, err = setConfig(r, key, value)
			}
		} else {
			output, err = getConfig(r, key)
		}
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			if len(res.Request().Arguments()) == 2 {
				return nil, nil // dont output anything
			}

			v := res.Output()
			if v == nil {
				k := res.Request().Arguments()[0]
				return nil, fmt.Errorf("config does not contain key: %s", k)
			}
			vf, ok := v.(*ConfigField)
			if !ok {
				return nil, u.ErrCast()
			}

			buf, err := config.HumanOutput(vf.Value)
			if err != nil {
				return nil, err
			}
			buf = append(buf, byte('\n'))
			return bytes.NewReader(buf), nil
		},
	},
	Type: ConfigField{},
	Subcommands: map[string]*cmds.Command{
		"show":    configShowCmd,
		"edit":    configEditCmd,
		"replace": configReplaceCmd,
	},
}

var configShowCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs the content of the config file.",
		ShortDescription: `
WARNING: Your private key is stored in the config file, and it will be
included in the output of this command.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		filename, err := config.Filename(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		output, err := showConfig(filename)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
}

var configEditCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Opens the config file for editing in $EDITOR.",
		ShortDescription: `
To use 'ipfs config edit', you must have the $EDITOR environment
variable set to your preferred text editor.
`,
	},

	Run: func(req cmds.Request, res cmds.Response) {
		filename, err := config.Filename(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = editConfig(filename)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}
	},
}

var configReplaceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Replaces the config with <file>.",
		ShortDescription: `
Make sure to back up the config file first if neccessary, as this operation
can't be undone.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "The file to use as the new config."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		r, err := fsrepo.Open(req.InvocContext().ConfigRoot)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer r.Close()

		file, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer file.Close()

		err = replaceConfig(r, file)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

func getConfig(r repo.Repo, key string) (*ConfigField, error) {
	value, err := r.GetConfigKey(key)
	if err != nil {
		return nil, fmt.Errorf("Failed to get config value: %q", err)
	}
	return &ConfigField{
		Key:   key,
		Value: value,
	}, nil
}

func setConfig(r repo.Repo, key string, value interface{}) (*ConfigField, error) {
	err := r.SetConfigKey(key, value)
	if err != nil {
		return nil, fmt.Errorf("Failed to set config value: %s (maybe use --json?)", err)
	}
	return getConfig(r, key)
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

func replaceConfig(r repo.Repo, file io.Reader) error {
	var cfg config.Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return errors.New("Failed to decode file as config")
	}

	return r.SetConfig(&cfg)
}
