package commands

import (
	"fmt"
	"io"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/coreapi"
	repo "github.com/ipfs/go-ipfs/repo"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	options "github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58/base58"
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
		"gen":      keyGenCmd,
		"export":   keyExportCmd,
		"import":   keyImportCmd,
		"identify": keyIdentifyCmd,
		"list":     keyListCmd,
		"rename":   keyRenameCmd,
		"rm":       keyRmCmd,
	},
}

type KeyOutput struct {
	Name string
	Id   string
}

type GenerateKeyOutput struct {
	Name string
	Id   string
	Sk   string
}

type ExportKeyOutput struct {
	Sk string
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
	keyExportOptionName    = "export"
	keyNoStoreOptionName   = "no-store"
)

var keyGenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new keypair",
	},
	Options: []cmds.Option{
		cmds.StringOption(keyStoreTypeOptionName, "t", "type of the key to create: rsa, ed25519").WithDefault("rsa"),
		cmds.IntOption(keyStoreSizeOptionName, "s", "size of the key to generate"),
		cmds.StringOption(keyFormatOptionName, "f", "output format: b58mh or b36cid").WithDefault("b58mh"),
		cmds.BoolOption(keyExportOptionName, "e", "return generated key for later re-import").WithDefault(false),
		cmds.BoolOption(keyNoStoreOptionName, "n", "don't add the key to the keychain").WithDefault(false),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", false, false, "name of key to create, required unless -n specified"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		typ, f := req.Options[keyStoreTypeOptionName].(string)
		if !f {
			return fmt.Errorf("please specify a key type with --type")
		}

		store := !req.Options[keyNoStoreOptionName].(bool)
		export := req.Options[keyExportOptionName].(bool)

		var name string
		var r repo.Repo = nil
		defer func() {
			if r != nil {
				r.Close()
			}
		}()
		if store {
			if len(req.Arguments) == 0 {
				return fmt.Errorf("you must specify a key name")
			}

			name = req.Arguments[0]

			if name == "self" {
				return fmt.Errorf("cannot create key with name 'self'")
			}

			cfgRoot, err := cmdenv.GetConfigRoot(env)
			if err != nil {
				return err
			}

			r, err = fsrepo.Open(cfgRoot)
			if err != nil {
				return err
			}

			_, err = r.Keystore().Get(name)
			if err == nil {
				return fmt.Errorf("key with name '%s' already exists", name)
			}
		}

		if !store && !export {
			return fmt.Errorf("you must export key if not storing")
		}

		opts := []options.KeyGenerateOption{options.Key.Type(typ)}

		size, sizefound := req.Options[keyStoreSizeOptionName].(int)
		if sizefound {
			opts = append(opts, options.Key.Size(size))
		}
		if err := verifyFormatLabel(req.Options[keyFormatOptionName].(string)); err != nil {
			return err
		}

		sk, pk, err := coreapi.GenerateKey(opts...)
		if err != nil {
			return err
		}

		if store {
			err = r.Keystore().Put(name, sk)
			if err != nil {
				return err
			}
		}

		pid, err := peer.IDFromPublicKey(pk)
		if err != nil {
			return err
		}

		var encoded string
		if export {
			encoded, err = encodeSKForExport(sk)
			if err != nil {
				return err
			}
		}

		return cmds.EmitOnce(res, &GenerateKeyOutput{
			Name: name,
			Id:   formatID(pid, req.Options[keyFormatOptionName].(string)),
			Sk:   encoded,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ko *GenerateKeyOutput) error {
			if ko.Sk != "" {
				_, err := w.Write([]byte(ko.Sk + "\n"))
				return err
			}
			_, err := w.Write([]byte(ko.Id + "\n"))
			return err
		}),
	},
	Type: GenerateKeyOutput{},
}

func verifyFormatLabel(formatLabel string) error {
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

func encodeSKForExport(sk crypto.PrivKey) (string, error) {
	data, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return "", err
	}
	return base58.Encode(data), nil
}

var keyExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Export a keypair",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to export").EnableStdin(),
	},
	Options: []cmds.Option{},
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

		encoded, err := encodeSKForExport(sk)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &ExportKeyOutput{
			Sk: encoded,
		})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, ko *ExportKeyOutput) error {
			_, err := w.Write([]byte(ko.Sk + "\n"))
			return err
		}),
	},
	Type: ExportKeyOutput{},
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
		cmds.StringArg("key", true, false, "key provided by generate or export"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		name := req.Arguments[0]

		if name == "self" {
			return fmt.Errorf("cannot import key with name 'self'")
		}

		encoded := req.Arguments[1]

		data, err := base58.Decode(encoded)
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

var keyIdentifyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Identify an exported keypair",
	},
	Options: []cmds.Option{
		cmds.StringOption(keyFormatOptionName, "f", "output format: b58mh or b36cid").WithDefault("b58mh"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "key provided by generate or export"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		encoded := req.Arguments[0]

		data, err := base58.Decode(encoded)
		if err != nil {
			return err
		}

		sk, err := crypto.UnmarshalPrivateKey(data)
		if err != nil {
			return err
		}

		pid, err := peer.IDFromPrivateKey(sk)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &KeyOutput{
			Name: "",
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
		if err := verifyFormatLabel(req.Options[keyFormatOptionName].(string)); err != nil {
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
