package commands

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	keystore "github.com/ipfs/boxo/keystore"
	cmds "github.com/ipfs/go-ipfs-cmds"
	oldcmds "github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/e"
	ke "github.com/ipfs/kubo/core/commands/keyencode"
	options "github.com/ipfs/kubo/core/coreiface/options"
	fsrepo "github.com/ipfs/kubo/repo/fsrepo"
	migrations "github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
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
		"rotate": keyRotateCmd,
		"sign":   keySignCmd,
		"verify": keyVerifyCmd,
	},
}

type KeyOutput struct {
	Name string
	Id   string //nolint
}

type KeyOutputList struct {
	Keys []KeyOutput
}

// KeyRenameOutput define the output type of keyRenameCmd
type KeyRenameOutput struct {
	Was       string
	Now       string
	Id        string //nolint
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
			return errors.New("please specify a key type with --type")
		}

		name := req.Arguments[0]
		if name == "self" {
			return errors.New("cannot create key with name 'self'")
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

const (
	// Key format options used both for importing and exporting.
	keyFormatOptionName            = "format"
	keyFormatPemCleartextOption    = "pem-pkcs8-cleartext"
	keyFormatLibp2pCleartextOption = "libp2p-protobuf-cleartext"
	keyAllowAnyTypeOptionName      = "allow-any-key-type"
)

var keyExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Export a keypair",
		ShortDescription: `
Exports a named libp2p key to disk.

By default, the output will be stored at './<key-name>.key', but an alternate
path can be specified with '--output=<path>' or '-o=<path>'.

It is possible to export a private key to interoperable PEM PKCS8 format by explicitly
passing '--format=pem-pkcs8-cleartext'. The resulting PEM file can then be consumed
elsewhere. For example, using openssl to get a PEM with public key:

  $ ipfs key export testkey --format=pem-pkcs8-cleartext -o privkey.pem
  $ openssl pkey -in privkey.pem -pubout > pubkey.pem
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to export").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(outputOptionName, "o", "The path where the output should be stored."),
		cmds.StringOption(keyFormatOptionName, "f", "The format of the exported private key, libp2p-protobuf-cleartext or pem-pkcs8-cleartext.").WithDefault(keyFormatLibp2pCleartextOption),
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

		// Check repo version, and error out if not matching
		ver, err := migrations.RepoVersion(cfgRoot)
		if err != nil {
			return err
		}
		if ver != fsrepo.RepoVersion {
			return fmt.Errorf("key export expects repo version (%d) but found (%d)", fsrepo.RepoVersion, ver)
		}

		// Export is read-only: safe to read it without acquiring repo lock
		// (this makes export work when ipfs daemon is already running)
		ksp := filepath.Join(cfgRoot, "keystore")
		ks, err := keystore.NewFSKeystore(ksp)
		if err != nil {
			return err
		}

		sk, err := ks.Get(name)
		if err != nil {
			return fmt.Errorf("key with name '%s' doesn't exist", name)
		}

		exportFormat, _ := req.Options[keyFormatOptionName].(string)
		var formattedKey []byte
		switch exportFormat {
		case keyFormatPemCleartextOption:
			stdKey, err := crypto.PrivKeyToStdKey(sk)
			if err != nil {
				return fmt.Errorf("converting libp2p private key to std Go key: %w", err)
			}
			// For some reason the ed25519.PrivateKey does not use pointer
			// receivers, so we need to convert it for MarshalPKCS8PrivateKey.
			// (We should probably change this upstream in PrivKeyToStdKey).
			if ed25519KeyPointer, ok := stdKey.(*ed25519.PrivateKey); ok {
				stdKey = *ed25519KeyPointer
			}
			// This function supports a restricted list of public key algorithms,
			// but we generate and use only the RSA and ed25519 types that are on that list.
			formattedKey, err = x509.MarshalPKCS8PrivateKey(stdKey)
			if err != nil {
				return fmt.Errorf("marshalling key to PKCS8 format: %w", err)
			}

		case keyFormatLibp2pCleartextOption:
			formattedKey, err = crypto.MarshalPrivateKey(sk)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unrecognized export format: %s", exportFormat)
		}

		return res.Emit(bytes.NewReader(formattedKey))
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
			exportFormat, _ := req.Options[keyFormatOptionName].(string)
			if outPath == "" {
				var fileExtension string
				switch exportFormat {
				case keyFormatPemCleartextOption:
					fileExtension = "pem"
				case keyFormatLibp2pCleartextOption:
					fileExtension = "key"
				}
				trimmed := strings.TrimRight(fmt.Sprintf("%s.%s", req.Arguments[0], fileExtension), "/")
				_, outPath = filepath.Split(trimmed)
				outPath = filepath.Clean(outPath)
			}

			// create file
			file, err := os.Create(outPath)
			if err != nil {
				return err
			}
			defer file.Close()

			switch exportFormat {
			case keyFormatPemCleartextOption:
				privKeyBytes, err := io.ReadAll(outReader)
				if err != nil {
					return err
				}

				err = pem.Encode(file, &pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privKeyBytes,
				})
				if err != nil {
					return fmt.Errorf("encoding PEM block: %w", err)
				}

			case keyFormatLibp2pCleartextOption:
				_, err = io.Copy(file, outReader)
				if err != nil {
					return err
				}
			}

			return nil
		},
	},
}

var keyImportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Import a key and prints imported key id",
		ShortDescription: `
Imports a key and stores it under the provided name.

By default, the key is assumed to be in 'libp2p-protobuf-cleartext' format,
however it is possible to import private keys wrapped in interoperable PEM PKCS8
by passing '--format=pem-pkcs8-cleartext'.

The PEM format allows for key generation outside of the IPFS node:

  $ openssl genpkey -algorithm ED25519 > ed25519.pem
  $ ipfs key import test-openssl -f pem-pkcs8-cleartext ed25519.pem
`,
	},
	Options: []cmds.Option{
		ke.OptionIPNSBase,
		cmds.StringOption(keyFormatOptionName, "f", "The format of the private key to import, libp2p-protobuf-cleartext or pem-pkcs8-cleartext.").WithDefault(keyFormatLibp2pCleartextOption),
		cmds.BoolOption(keyAllowAnyTypeOptionName, "Allow importing any key type.").WithDefault(false),
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

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		importFormat, _ := req.Options[keyFormatOptionName].(string)
		var sk crypto.PrivKey
		switch importFormat {
		case keyFormatPemCleartextOption:
			pemBlock, rest := pem.Decode(data)
			if pemBlock == nil {
				return fmt.Errorf("PEM block not found in input data:\n%s", rest)
			}

			if pemBlock.Type != "PRIVATE KEY" {
				return fmt.Errorf("expected PRIVATE KEY type in PEM block but got: %s", pemBlock.Type)
			}

			stdKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
			if err != nil {
				return fmt.Errorf("parsing PKCS8 format: %w", err)
			}

			// In case ed25519.PrivateKey is returned we need the pointer for
			// conversion to libp2p (see export command for more details).
			if ed25519KeyPointer, ok := stdKey.(ed25519.PrivateKey); ok {
				stdKey = &ed25519KeyPointer
			}

			sk, _, err = crypto.KeyPairFromStdKey(stdKey)
			if err != nil {
				return fmt.Errorf("converting std Go key to libp2p key: %w", err)
			}
		case keyFormatLibp2pCleartextOption:
			sk, err = crypto.UnmarshalPrivateKey(data)
			if err != nil {
				// check if data is PEM, if so, provide user with hint
				pemBlock, _ := pem.Decode(data)
				if pemBlock != nil {
					return fmt.Errorf("unexpected PEM block for format=%s: try again with format=%s", keyFormatLibp2pCleartextOption, keyFormatPemCleartextOption)
				}
				return fmt.Errorf("unable to unmarshall format=%s: %w", keyFormatLibp2pCleartextOption, err)
			}

		default:
			return fmt.Errorf("unrecognized import format: %s", importFormat)
		}

		// We only allow importing keys of the same type we generate (see list in
		// https://github.com/ipfs/interface-go-ipfs-core/blob/1c3d8fc/options/key.go#L58-L60),
		// unless explicitly stated by the user.
		allowAnyKeyType, _ := req.Options[keyAllowAnyTypeOptionName].(bool)
		if !allowAnyKeyType {
			switch t := sk.(type) {
			case *crypto.RsaPrivateKey, *crypto.Ed25519PrivateKey:
			default:
				return fmt.Errorf("key type %T is not allowed to be imported, only RSA or Ed25519;"+
					" use flag --%s if you are sure of what you're doing",
					t, keyAllowAnyTypeOptionName)
			}
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
		Tagline: "List all local keypairs.",
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
		Tagline: "Rename a keypair.",
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
		Tagline: "Remove a keypair.",
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
		Tagline: "Rotates the IPFS identity.",
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
	PreRun:   DaemonNotRunning,
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

type KeySignOutput struct {
	Key       KeyOutput
	Signature string
}

var keySignCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Generates a signature for the given data with a specified key. Useful for proving the key ownership.",
		LongDescription: `
Sign arbitrary bytes, such as to prove ownership of a Peer ID or an IPNS Name.
To avoid signature reuse, the signed payload is always prefixed with
"libp2p-key signed message:".
`,
	},
	Options: []cmds.Option{
		cmds.StringOption("key", "k", "The name of the key to use for signing."),
		ke.OptionIPNSBase,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "The data to sign.").EnableStdin(),
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

		name, _ := req.Options["key"].(string)

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		key, signature, err := api.Key().Sign(req.Context, name, data)
		if err != nil {
			return err
		}

		encodedSignature, err := mbase.Encode(mbase.Base64url, signature)
		if err != nil {
			return err
		}

		return res.Emit(&KeySignOutput{
			Key: KeyOutput{
				Name: key.Name(),
				Id:   keyEnc.FormatID(key.ID()),
			},
			Signature: encodedSignature,
		})
	},
	Type: KeySignOutput{},
}

type KeyVerifyOutput struct {
	Key            KeyOutput
	SignatureValid bool
}

var keyVerifyCmd = &cmds.Command{
	Status: cmds.Experimental,
	Helptext: cmds.HelpText{
		Tagline: "Verify that the given data and signature match.",
		LongDescription: `
Verify if the given data and signatures match. To avoid the signature reuse,
the signed payload is always prefixed with "libp2p-key signed message:".
`,
	},
	Options: []cmds.Option{
		cmds.StringOption("key", "k", "The name of the key to use for signing."),
		cmds.StringOption("signature", "s", "Multibase-encoded signature to verify."),
		ke.OptionIPNSBase,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "The data to verify against the given signature.").EnableStdin(),
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

		name, _ := req.Options["key"].(string)
		encodedSignature, _ := req.Options["signature"].(string)

		_, signature, err := mbase.Decode(encodedSignature)
		if err != nil {
			return err
		}

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			return err
		}

		key, valid, err := api.Key().Verify(req.Context, name, signature, data)
		if err != nil {
			return err
		}

		return res.Emit(&KeyVerifyOutput{
			Key: KeyOutput{
				Name: key.Name(),
				Id:   keyEnc.FormatID(key.ID()),
			},
			SignatureValid: valid,
		})
	},
	Type: KeyVerifyOutput{},
}

// DaemonNotRunning checks to see if the ipfs repo is locked, indicating that
// the daemon is running, and returns and error if the daemon is running.
func DaemonNotRunning(req *cmds.Request, env cmds.Environment) error {
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
}
