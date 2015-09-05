package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	assets "github.com/ipfs/go-ipfs/assets"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	namesys "github.com/ipfs/go-ipfs/namesys"
	config "github.com/ipfs/go-ipfs/repo/config"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
)

const nBitsForKeypairDefault = 2048

var initCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Initializes IPFS config file",
		ShortDescription: "Initializes IPFS configuration files and generates a new keypair.",
	},

	Options: []cmds.Option{
		cmds.IntOption("bits", "b", fmt.Sprintf("Number of bits to use in the generated RSA private key (defaults to %d)", nBitsForKeypairDefault)),
		cmds.BoolOption("force", "f", "Overwrite existing config (if it exists)"),
		cmds.BoolOption("empty-repo", "e", "Don't add and pin help files to the local storage"),

		// TODO need to decide whether to expose the override as a file or a
		// directory. That is: should we allow the user to also specify the
		// name of the file?
		// TODO cmds.StringOption("event-logs", "l", "Location for machine-readable event logs"),
	},
	PreRun: func(req cmds.Request) error {
		daemonLocked, err := fsrepo.LockedByOtherProcess(req.InvocContext().ConfigRoot)
		if err != nil {
			return err
		}

		log.Info("checking if daemon is running...")
		if daemonLocked {
			e := "ipfs daemon is running. please stop it to run this command"
			return cmds.ClientError(e)
		}

		return nil
	},
	Run: func(req cmds.Request, res cmds.Response) {
		if req.InvocContext().Online {
			res.SetError(errors.New("init must be run offline only!"), cmds.ErrNormal)
			return
		}

		force, _, err := req.Option("f").Bool() // if !found, it's okay force == false
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		empty, _, err := req.Option("e").Bool() // if !empty, it's okay empty == false
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		nBitsForKeypair, bitsOptFound, err := req.Option("b").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !bitsOptFound {
			nBitsForKeypair = nBitsForKeypairDefault
		}

		if err := doInit(os.Stdout, req.InvocContext().ConfigRoot, force, empty, nBitsForKeypair); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var errRepoExists = errors.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

func initWithDefaults(out io.Writer, repoRoot string) error {
	return doInit(out, repoRoot, false, false, nBitsForKeypairDefault)
}

func doInit(out io.Writer, repoRoot string, force bool, empty bool, nBitsForKeypair int) error {
	if _, err := fmt.Fprintf(out, "initializing ipfs node at %s\n", repoRoot); err != nil {
		return err
	}

	if err := checkWriteable(repoRoot); err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) && !force {
		return errRepoExists
	}

	conf, err := config.Init(out, nBitsForKeypair)
	if err != nil {
		return err
	}

	if fsrepo.IsInitialized(repoRoot) {
		if err := fsrepo.Remove(repoRoot); err != nil {
			return err
		}
	}

	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return err
	}

	if !empty {
		if err := addDefaultAssets(out, repoRoot); err != nil {
			return err
		}
	}

	return initializeIpnsKeyspace(repoRoot)
}

func checkWriteable(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		// dir exists, make sure we can write to it
		testfile := path.Join(dir, "test")
		fi, err := os.Create(testfile)
		if err != nil {
			if os.IsPermission(err) {
				return fmt.Errorf("%s is not writeable by the current user", dir)
			}
			return fmt.Errorf("unexpected error while checking writeablility of repo root: %s", err)
		}
		fi.Close()
		return os.Remove(testfile)
	}

	if os.IsNotExist(err) {
		// dir doesnt exist, check that we can create it
		return os.Mkdir(dir, 0775)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("cannot write to %s, incorrect permissions", err)
	}

	return err
}

func addDefaultAssets(out io.Writer, repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	dkey, err := assets.SeedInitDocs(nd)
	if err != nil {
		return fmt.Errorf("init: seeding init docs failed: %s", err)
	}
	log.Debugf("init: seeded init docs %s", dkey)

	if _, err = fmt.Fprintf(out, "to get started, enter:\n"); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "\n\tipfs cat /ipfs/%s/readme\n\n", dkey)
	return err
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := fsrepo.Open(repoRoot)
	if err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewNode(ctx, &core.BuildCfg{Repo: r})
	if err != nil {
		return err
	}
	defer nd.Close()

	err = nd.SetupOfflineRouting()
	if err != nil {
		return err
	}

	return namesys.InitializeKeyspace(ctx, nd.DAG, nd.Namesys, nd.Pinning, nd.PrivateKey)
}
