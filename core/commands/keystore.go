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
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	mbase "github.com/multiformats/go-multibase"
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
	keyStoreTypeOptionName = "type"
	keyStoreSizeOptionName = "size"
	keyFormatOptionName    = "format"
)

var keyGenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new keypair",
	},
	Options: []cmds.Option{
		cmds.StringOption(keyStoreTypeOptionName, "t", "type of the key to create: rsa, ed25519").WithDefault("rsa"),
		cmds.IntOption(keyStoreSizeOptionName, "s", "size of the key to generate"),
		cmds.StringOption(keyFormatOptionName, "f", "output format: b58mh or b36cid").WithDefault("b58mh"),
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
		if err = verifyIDFormatLabel(req.Options[keyFormatOptionName].(string)); err != nil {
			return err
		}

		key, err := api.Key().Generate(req.Context, name, opts...)

		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyOutput{
			Name: name,
			Id:   formatID(key.ID(), req.Options[keyFormatOptionName].(string)),
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
		cmds.StringOption(keyFormatOptionName, "f", "output format: b58mh or b36cid").WithDefault("b58mh"),
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
			Id:   formatID(pid, req.Options[keyFormatOptionName].(string)),
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
		cmds.StringOption(keyFormatOptionName, "f", "output format: b58mh or b36cid").WithDefault("b58mh"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		if err := verifyIDFormatLabel(req.Options[keyFormatOptionName].(string)); err != nil {
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
				Id:   formatID(key.ID(), req.Options[keyFormatOptionName].(string)),
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
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
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
			Id:        key.ID().Pretty(),
			Overwrite: overwritten,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, kro *KeyRenameOutput) error {
			if kro.Overwrite {
				fmt.Fprintf(w, "Key %s renamed to %s with overwriting\n", kro.Id, kro.Now)
			} else {
				fmt.Fprintf(w, "Key %s renamed to %s\n", kro.Id, kro.Now)
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
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
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

			list = append(list, KeyOutput{Name: name, Id: key.ID().Pretty()})
		}

		return cmds.EmitOnce(res, &KeyOutputList{list})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: keyOutputListEncoders(),
	},
	Type: KeyOutputList{},
}

func verifyIDFormatLabel(formatLabel string) error {
	switch formatLabel {
	case "b58mh":
		return nil
	case "b36cid":
		return nil
	}
	return fmt.Errorf("invalid output format option")
}

func formatID(id peer.ID, formatLabel string) string {
	switch formatLabel {
	case "b58mh":
		return id.Pretty()
	case "b36cid":
		if s, err := peer.ToCid(id).StringOfBase(mbase.Base36); err != nil {
			panic(err)
		} else {
			return s
		}
	default:
		panic("unreachable")
	}
}

func keyOutputListEncoders() cmds.EncoderFunc {
	return cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, list *KeyOutputList) error {
		withID, _ := req.Options["l"].(bool)

		tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
		for _, s := range list.Keys {
			if withID {
				fmt.Fprintf(tw, "%s\t%s\t\n", s.Id, s.Name)
			} else {
				fmt.Fprintf(tw, "%s\n", s.Name)
			}
		}
		tw.Flush()
		return nil
	})
}
