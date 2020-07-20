package main

import (
	"fmt"
	"io"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	config "github.com/ipfs/go-ipfs-config"
	oldcmds "github.com/ipfs/go-ipfs/commands"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipfs/interface-go-ipfs-core/options"
)

const (
	oldKeyOptionName = "oldkey"
)

var rotateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Rotates the ipfs identity.",
		ShortDescription: `
Generates a new ipfs identity and saves it to the ipfs config file.
The daemon must not be running when calling this command.

ipfs uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

    export IPFS_PATH=/path/to/ipfsrepo
`,
	},
	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringOption(oldKeyOptionName, "o", "Keystore name for the old/rotated-out key."),
		cmds.StringOption(algorithmOptionName, "a", "Cryptographic algorithm to use for key generation.").WithDefault(algorithmDefault),
		cmds.IntOption(bitsOptionName, "b", "Number of bits to use in the generated RSA private key."),
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
		nBitsForKeypair, nBitsGiven := req.Options[bitsOptionName].(int)
		algorithm, _ := req.Options[algorithmOptionName].(string)
		oldKey, ok := req.Options[oldKeyOptionName].(string)
		if !ok {
			return fmt.Errorf("keystore name for backing up old key must be provided")
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
