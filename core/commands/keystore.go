package commands

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"
	ke "github.com/ipfs/go-ipfs/core/commands/keyencode"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

var KeyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create and list IPNS name keypairs",
		ShortDescription: `
'ipfs key gen' generates a new keypair for usage with IPNS and 'ipfs name
publish'.

  > ipfs key gen --type=rsa --size=2048 mykey
  > ipfs name publish --key=mykey QmSomeHash

'ipfs key list' lists the available keys.

  > ipfs key list
  self
  mykey
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"gen":    keyGenCmd,
		"export": keyExportCmd,
		"import": keyImportCmd,
		"list":   keyListCmd,
		"rename": keyRenameCmd,
		"rm":     keyRmCmd,
		"rotate": keyRotateCmd,
	},
}

type KeyOutput struct {
	Name string
	Id   string
}

type KeyOutputList struct {
	Keys []KeyOutput
}

// KeyRenameOutput define the output type of keyRenameCmd
type KeyRenameOutput struct {
	Was       string
	Now       string
	Id        string
	Overwrite bool
}

const (
	keyStoreAlgorithmDefault = options.Ed25519Key
	keyStoreTypeOptionName   = "type"
	keyStoreSizeOptionName   = "size"
	oldKeyOptionName         = "oldkey"
)

var keyGenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new keypair",
	},
	Options: []cmds.Option{
		cmds.StringOption(keyStoreTypeOptionName, "t", "type of the key to create: rsa, ed25519").WithDefault(keyStoreAlgorithmDefault),
		cmds.IntOption(keyStoreSizeOptionName, "s", "size of the key to generate"),
		ke.OptionIPNSBase,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to create"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		typ, f := req.Options[keyStoreTypeOptionName].(string)
		if !f {
			return fmt.Errorf("please specify a key type with --type")
		}

		name := req.Arguments[0]
		if name == "self" {
			return fmt.Errorf("cannot create key with name 'self'")
		}

		opts := []options.KeyGenerateOption{options.Key.Type(typ)}

		size, sizefound := req.Options[keyStoreSizeOptionName].(int)
		if sizefound {
			opts = append(opts, options.Key.Size(size))
		}
		keyEnc, err := ke.KeyEncoderFromString(req.Options[ke.OptionIPNSBase.Name()].(string))
		if err != nil {
			return err
		}

		key, err := api.Key().Generate(req.Context, name, opts...)

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyOutput{
			Name: name,
			Id:   keyEnc.FormatID(key.ID()),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ko *KeyOutput) error {
			_, err := w.Write([]byte(ko.Id + "\n"))
			return err
		}),
	},
	Type: KeyOutput{},
}

var keyExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Export a keypair",
		ShortDescription: `
Exports a named libp2p key to disk.

By default, the output will be stored at './<key-name>.key', but an alternate
path can be specified with '--output=<path>' or '-o=<path>'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to export").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(outputOptionName, "o", "The path where the output should be stored."),
	},
	NoRemote: true,
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		name := req.Arguments[0]

		if name == "self" {
			return fmt.Errorf("cannot export key with name 'self'")
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

		sk, err := r.Keystore().Get(name)
		if err != nil {
			return fmt.Errorf("key with name '%s' doesn't exist", name)
		}

		encoded, err := crypto.MarshalPrivateKey(sk)
		if err != nil {
			return err
		}

		return res.Emit(bytes.NewReader(encoded))
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			req := res.Request()

			v, err := res.Next()
			if err != nil {
				return err
			}

			outReader, ok := v.(io.Reader)
			if !ok {
				return e.New(e.TypeErr(outReader, v))
			}

			outPath, _ := req.Options[outputOptionName].(string)
			if outPath == "" {
				trimmed := strings.TrimRight(fmt.Sprintf("%s.key", req.Arguments[0]), "/")
				_, outPath = filepath.Split(trimmed)
				outPath = filepath.Clean(outPath)
			}

			// create file
			file, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(file, outReader)
			if err != nil {
				return err
			}

			return nil
		},
	},
}

var keyImportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Import a key and prints imported key id",
	},
	Options: []cmds.Option{
		ke.OptionIPNSBase,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name to associate with key in keychain"),
		cmds.FileArg("key", true, false, "key provided by generate or export"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		name := req.Arguments[0]

		if name == "self" {
			return fmt.Errorf("cannot import key with name 'self'")
		}

		keyEnc, err := ke.KeyEncoderFromString(req.Options[ke.OptionIPNSBase.Name()].(string))
		if err != nil {
			return err
		}

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		sk, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return err
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

		_, err = r.Keystore().Get(name)
		if err == nil {
			return fmt.Errorf("key with name '%s' already exists", name)
		}

		err = r.Keystore().Put(name, sk)
		if err != nil {
			return err
		}

		pid, err := peer.IDFromPrivateKey(sk)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyOutput{
			Name: name,
			Id:   keyEnc.FormatID(pid),
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ko *KeyOutput) error {
			_, err := w.Write([]byte(ko.Id + "\n"))
			return err
		}),
	},
	Type: KeyOutput{},
}

var keyListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List all local keypairs",
	},
	Options: []cmds.Option{
		cmds.BoolOption("l", "Show extra information about keys."),
		ke.OptionIPNSBase,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		keyEnc, err := ke.KeyEncoderFromString(req.Options[ke.OptionIPNSBase.Name()].(string))
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		keys, err := api.Key().List(req.Context)
		if err != nil {
			return err
		}

		list := make([]KeyOutput, 0, len(keys))

		for _, key := range keys {
			list = append(list, KeyOutput{
				Name: key.Name(),
				Id:   keyEnc.FormatID(key.ID()),
			})
		}

		return cmds.EmitOnce(res, &KeyOutputList{list})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: keyOutputListEncoders(),
	},
	Type: KeyOutputList{},
}

const (
	keyStoreForceOptionName = "force"
)

var keyRenameCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Rename a keypair",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to rename"),
		cmds.StringArg("newName", true, false, "new name of the key"),
	},
	Options: []cmds.Option{
		cmds.BoolOption(keyStoreForceOptionName, "f", "Allow to overwrite an existing key."),
		ke.OptionIPNSBase,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		keyEnc, err := ke.KeyEncoderFromString(req.Options[ke.OptionIPNSBase.Name()].(string))
		if err != nil {
			return err
		}

		name := req.Arguments[0]
		newName := req.Arguments[1]
		force, _ := req.Options[keyStoreForceOptionName].(bool)

		key, overwritten, err := api.Key().Rename(req.Context, name, newName, options.Key.Force(force))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyRenameOutput{
			Was:       name,
			Now:       newName,
			Id:        keyEnc.FormatID(key.ID()),
			Overwrite: overwritten,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, kro *KeyRenameOutput) error {
			if kro.Overwrite {
				fmt.Fprintf(w, "Key %s renamed to %s with overwriting\n", kro.Id, cmdenv.EscNonPrint(kro.Now))
			} else {
				fmt.Fprintf(w, "Key %s renamed to %s\n", kro.Id, cmdenv.EscNonPrint(kro.Now))
			}
			return nil
		}),
	},
	Type: KeyRenameOutput{},
}

var keyRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove a keypair",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, true, "names of keys to remove").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("l", "Show extra information about keys."),
		ke.OptionIPNSBase,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}
		keyEnc, err := ke.KeyEncoderFromString(req.Options[ke.OptionIPNSBase.Name()].(string))
		if err != nil {
			return err
		}

		names := req.Arguments

		list := make([]KeyOutput, 0, len(names))
		for _, name := range names {
			key, err := api.Key().Remove(req.Context, name)
			if err != nil {
				return err
			}

			list = append(list, KeyOutput{
				Name: name,
				Id:   keyEnc.FormatID(key.ID()),
			})
		}

		return cmds.EmitOnce(res, &KeyOutputList{list})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: keyOutputListEncoders(),
	},
	Type: KeyOutputList{},
}

var keyRotateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Rotates the ipfs identity.",
		ShortDescription: `
Generates a new ipfs identity and saves it to the ipfs config file.
Your existing identity key will be backed up in the Keystore.
The daemon must not be running when calling this command.

ipfs uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

    export IPFS_PATH=/path/to/ipfsrepo
`,
	},
	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringOption(oldKeyOptionName, "o", "Keystore name to use for backing up your existing identity"),
		cmds.StringOption(keyStoreTypeOptionName, "t", "type of the key to create: rsa, ed25519").WithDefault(keyStoreAlgorithmDefault),
		cmds.IntOption(keyStoreSizeOptionName, "s", "size of the key to generate"),
	},
	NoRemote: true,
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		daemonLocked, err := fsrepo.LockedByOtherProcess(cctx.ConfigRoot)
		if err != nil {
			return err
		}

		log.Info("checking if daemon is running...")
		if daemonLocked {
			log.Debug("ipfs daemon is running")
			e := "ipfs daemon is running. please stop it to run this command"
			return cmds.ClientError(e)
		}

		return nil
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cctx := env.(*oldcmds.Context)
		nBitsForKeypair, nBitsGiven := req.Options[keyStoreSizeOptionName].(int)
		algorithm, _ := req.Options[keyStoreTypeOptionName].(string)
		oldKey, ok := req.Options[oldKeyOptionName].(string)
		if !ok {
			return fmt.Errorf("keystore name for backing up old key must be provided")
		}
		if oldKey == "self" {
			return fmt.Errorf("keystore name for back up cannot be named 'self'")
		}
		return doRotate(os.Stdout, cctx.ConfigRoot, oldKey, algorithm, nBitsForKeypair, nBitsGiven)
	},
}

func doRotate(out io.Writer, repoRoot string, oldKey string, algorithm string, nBitsForKeypair int, nBitsGiven bool) error {
	// Open repo
	repo, err := fsrepo.Open(repoRoot)
	if err != nil {
		return fmt.Errorf("opening repo (%v)", err)
	}
	defer repo.Close()

	// Read config file from repo
	cfg, err := repo.Config()
	if err != nil {
		return fmt.Errorf("reading config from repo (%v)", err)
	}

	// Generate new identity
	var identity config.Identity
	if nBitsGiven {
		identity, err = config.CreateIdentity(out, []options.KeyGenerateOption{
			options.Key.Size(nBitsForKeypair),
			options.Key.Type(algorithm),
		})
	} else {
		identity, err = config.CreateIdentity(out, []options.KeyGenerateOption{
			options.Key.Type(algorithm),
		})
	}
	if err != nil {
		return fmt.Errorf("creating identity (%v)", err)
	}

	// Save old identity to keystore
	oldPrivKey, err := cfg.Identity.DecodePrivateKey("")
	if err != nil {
		return fmt.Errorf("decoding old private key (%v)", err)
	}
	keystore := repo.Keystore()
	if err := keystore.Put(oldKey, oldPrivKey); err != nil {
		return fmt.Errorf("saving old key in keystore (%v)", err)
	}

	// Update identity
	cfg.Identity = identity

	// Write config file to repo
	if err = repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("saving new key to config (%v)", err)
	}
	return nil
}

func keyOutputListEncoders() cmds.EncoderFunc {
	return cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *KeyOutputList) error {
		withID, _ := req.Options["l"].(bool)

		tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
		for _, s := range list.Keys {
			if withID {
				fmt.Fprintf(tw, "%s\t%s\t\n", s.Id, cmdenv.EscNonPrint(s.Name))
			} else {
				fmt.Fprintf(tw, "%s\n", cmdenv.EscNonPrint(s.Name))
			}
		}
		tw.Flush()
		return nil
	})
}
