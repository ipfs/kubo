package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"

	"github.com/elgris/jsondiff"
	"github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs-config"
)

// ConfigUpdateOutput is config profile apply command's output
type ConfigUpdateOutput struct {
	OldCfg map[string]interface{}
	NewCfg map[string]interface{}
}

type ConfigField struct {
	Key   string
	Value interface{}
}

const (
	configBoolOptionName   = "bool"
	configJSONOptionName   = "json"
	configDryRunOptionName = "dry-run"
)

var ConfigCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and set ipfs config values.",
		ShortDescription: `
'ipfs config' controls configuration variables. It works like 'git config'.
The configuration values are stored in a config file inside your ipfs
repository.`,
		LongDescription: `
'ipfs config' controls configuration variables. It works
much like 'git config'. The configuration values are stored in a config
file inside your IPFS repository.

Examples:

Get the value of the 'Datastore.Path' key:

  $ ipfs config Datastore.Path

Set the value of the 'Datastore.Path' key:

  $ ipfs config Datastore.Path ~/.ipfs/datastore
`,
	},
	Subcommands: map[string]*cmds.Command{
		"show":    configShowCmd,
		"edit":    configEditCmd,
		"replace": configReplaceCmd,
		"profile": configProfileCmd,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key of the config entry (e.g. \"Addresses.API\")."),
		cmds.StringArg("value", false, false, "The value to set the config entry to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(configBoolOptionName, "Set a boolean value."),
		cmds.BoolOption(configJSONOptionName, "Parse stringified JSON."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		args := req.Arguments
		key := args[0]

		var output *ConfigField

		// This is a temporary fix until we move the private key out of the config file
		switch strings.ToLower(key) {
		case "identity", "identity.privkey":
			return errors.New("cannot show or change private key through API")
		default:
		}

		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}
		r, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer r.Close()
		if len(args) == 2 {
			value := args[1]

			if parseJSON, _ := req.Options[configJSONOptionName].(bool); parseJSON {
				var jsonVal interface{}
				if err := json.Unmarshal([]byte(value), &jsonVal); err != nil {
					err = fmt.Errorf("failed to unmarshal json. %s", err)
					return err
				}

				output, err = setConfig(r, key, jsonVal)
			} else if isbool, _ := req.Options[configBoolOptionName].(bool); isbool {
				output, err = setConfig(r, key, value == "true")
			} else {
				output, err = setConfig(r, key, value)
			}
		} else {
			output, err = getConfig(r, key)
		}

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ConfigField) error {
			if len(req.Arguments) == 2 {
				return nil
			}

			buf, err := config.HumanOutput(out.Value)
			if err != nil {
				return err
			}
			buf = append(buf, byte('\n'))

			_, err = w.Write(buf)
			return err
		}),
	},
	Type: ConfigField{},
}

var configShowCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Output config file contents.",
		ShortDescription: `
NOTE: For security reasons, this command will omit your private key. If you would like to make a full backup of your config (private key included), you must copy the config file from your repo.
`,
	},
	Type: map[string]interface{}{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		fname, err := config.Filename(cfgRoot)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadFile(fname)
		if err != nil {
			return err
		}

		var cfg map[string]interface{}
		err = json.Unmarshal(data, &cfg)
		if err != nil {
			return err
		}

		err = scrubValue(cfg, []string{config.IdentityTag, config.PrivKeyTag})
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &cfg)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *map[string]interface{}) error {
			buf, err := config.HumanOutput(out)
			if err != nil {
				return err
			}
			buf = append(buf, byte('\n'))
			_, err = w.Write(buf)
			return err
		}),
	},
}

func scrubValue(m map[string]interface{}, key []string) error {
	find := func(m map[string]interface{}, k string) (string, interface{}, bool) {
		lckey := strings.ToLower(k)
		for mkey, val := range m {
			lcmkey := strings.ToLower(mkey)
			if lckey == lcmkey {
				return mkey, val, true
			}
		}
		return "", nil, false
	}

	cur := m
	for _, k := range key[:len(key)-1] {
		foundk, val, ok := find(cur, k)
		if !ok {
			return errors.New("failed to find specified key")
		}

		if foundk != k {
			// case mismatch, calling this an error
			return fmt.Errorf("case mismatch in config, expected %q but got %q", k, foundk)
		}

		mval, mok := val.(map[string]interface{})
		if !mok {
			return fmt.Errorf("%s was not a map", foundk)
		}

		cur = mval
	}

	todel, _, ok := find(cur, key[len(key)-1])
	if !ok {
		return fmt.Errorf("%s, not found", strings.Join(key, "."))
	}

	delete(cur, todel)
	return nil
}

var configEditCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Open the config file for editing in $EDITOR.",
		ShortDescription: `
To use 'ipfs config edit', you must have the $EDITOR environment
variable set to your preferred text editor.
`,
	},

	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		filename, err := config.Filename(cfgRoot)
		if err != nil {
			return err
		}

		return editConfig(filename)
	},
}

var configReplaceCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Replace the config with <file>.",
		ShortDescription: `
Make sure to back up the config file first if necessary, as this operation
can't be undone.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "The file to use as the new config."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		r, err := fsrepo.Open(cfgRoot)
		if err != nil {
			return err
		}
		defer r.Close()

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		return replaceConfig(r, file)
	},
}

var configProfileCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Apply profiles to config.",
		ShortDescription: fmt.Sprintf(`
Available profiles:
%s
`, buildProfileHelp()),
	},

	Subcommands: map[string]*cmds.Command{
		"apply": configProfileApplyCmd,
	},
}

var configProfileApplyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Apply profile to config.",
	},
	Options: []cmds.Option{
		cmds.BoolOption(configDryRunOptionName, "print difference between the current config and the config that would be generated"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("profile", true, false, "The profile to apply to the config."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		profile, ok := config.Profiles[req.Arguments[0]]
		if !ok {
			return fmt.Errorf("%s is not a profile", req.Arguments[0])
		}

		dryRun, _ := req.Options[configDryRunOptionName].(bool)
		cfgRoot, err := cmdenv.GetConfigRoot(env)
		if err != nil {
			return err
		}

		oldCfg, newCfg, err := transformConfig(cfgRoot, req.Arguments[0], profile.Transform, dryRun)
		if err != nil {
			return err
		}

		oldCfgMap, err := scrubPrivKey(oldCfg)
		if err != nil {
			return err
		}

		newCfgMap, err := scrubPrivKey(newCfg)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ConfigUpdateOutput{
			OldCfg: oldCfgMap,
			NewCfg: newCfgMap,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ConfigUpdateOutput) error {
			diff := jsondiff.Compare(out.OldCfg, out.NewCfg)
			buf := jsondiff.Format(diff)

			_, err := w.Write(buf)
			return err
		}),
	},
	Type: ConfigUpdateOutput{},
}

func buildProfileHelp() string {
	var out string

	for name, profile := range config.Profiles {
		dlines := strings.Split(profile.Description, "\n")
		for i := range dlines {
			dlines[i] = "    " + dlines[i]
		}

		out = out + fmt.Sprintf("  '%s':\n%s\n", name, strings.Join(dlines, "\n"))
	}

	return out
}

// scrubPrivKey scrubs private key for security reasons.
func scrubPrivKey(cfg *config.Config) (map[string]interface{}, error) {
	cfgMap, err := config.ToMap(cfg)
	if err != nil {
		return nil, err
	}

	err = scrubValue(cfgMap, []string{config.IdentityTag, config.PrivKeyTag})
	if err != nil {
		return nil, err
	}

	return cfgMap, nil
}

// transformConfig returns old config and new config instead of difference between they,
// because apply command can provide stable API through this way.
// If dryRun is true, repo's config should not be updated and persisted
// to storage. Otherwise, repo's config should be updated and persisted
// to storage.
func transformConfig(configRoot string, configName string, transformer config.Transformer, dryRun bool) (*config.Config, *config.Config, error) {
	r, err := fsrepo.Open(configRoot)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()

	oldCfg, err := r.Config()
	if err != nil {
		return nil, nil, err
	}

	// make a copy to avoid updating repo's config unintentionally
	newCfg, err := oldCfg.Clone()
	if err != nil {
		return nil, nil, err
	}

	err = transformer(newCfg)
	if err != nil {
		return nil, nil, err
	}

	if !dryRun {
		_, err = r.BackupConfig("pre-" + configName + "-")
		if err != nil {
			return nil, nil, err
		}

		err = r.SetConfig(newCfg)
		if err != nil {
			return nil, nil, err
		}
	}

	return oldCfg, newCfg, nil
}

func getConfig(r repo.Repo, key string) (*ConfigField, error) {
	value, err := r.GetConfigKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get config value: %q", err)
	}
	return &ConfigField{
		Key:   key,
		Value: value,
	}, nil
}

func setConfig(r repo.Repo, key string, value interface{}) (*ConfigField, error) {
	err := r.SetConfigKey(key, value)
	if err != nil {
		return nil, fmt.Errorf("failed to set config value: %s (maybe use --json?)", err)
	}
	return getConfig(r, key)
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
		return errors.New("failed to decode file as config")
	}
	if len(cfg.Identity.PrivKey) != 0 {
		return errors.New("setting private key with API is not supported")
	}

	keyF, err := getConfig(r, config.PrivKeySelector)
	if err != nil {
		return errors.New("failed to get PrivKey")
	}

	pkstr, ok := keyF.Value.(string)
	if !ok {
		return errors.New("private key in config was not a string")
	}

	cfg.Identity.PrivKey = pkstr

	return r.SetConfig(&cfg)
}
