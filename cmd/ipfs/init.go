package main

import (
	"bytes"
	"fmt"
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	assets "github.com/jbenet/go-ipfs/assets"
	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	coreunix "github.com/jbenet/go-ipfs/core/coreunix"
	namesys "github.com/jbenet/go-ipfs/namesys"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
	debugerror "github.com/jbenet/go-ipfs/util/debugerror"
)

const nBitsForKeypairDefault = 4096

var initCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Initializes IPFS config file",
		ShortDescription: "Initializes IPFS configuration files and generates a new keypair.",
	},

	Options: []cmds.Option{
		cmds.IntOption("bits", "b", "Number of bits to use in the generated RSA private key (defaults to 4096)"),
		cmds.StringOption("passphrase", "p", "Passphrase for encrypting the private key"),
		cmds.BoolOption("force", "f", "Overwrite existing config (if it exists)"),

		// TODO need to decide whether to expose the override as a file or a
		// directory. That is: should we allow the user to also specify the
		// name of the file?
		// TODO cmds.StringOption("event-logs", "l", "Location for machine-readable event logs"),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		force, _, err := req.Option("f").Bool() // if !found, it's okay force == false
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

		rpipe, wpipe := io.Pipe()
		go func() {
			defer wpipe.Close()
			if err := doInit(wpipe, req.Context().ConfigRoot, force, nBitsForKeypair); err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}()
		res.SetOutput(rpipe)
	},
}

var errRepoExists = debugerror.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

func initWithDefaults(out io.Writer, repoRoot string) error {
	err := doInit(out, repoRoot, false, nBitsForKeypairDefault)
	return debugerror.Wrap(err)
}

func writef(out io.Writer, format string, ifs ...interface{}) error {
	_, err := out.Write([]byte(fmt.Sprintf(format, ifs...)))
	return err
}

func doInit(out io.Writer, repoRoot string, force bool, nBitsForKeypair int) error {
	if err := writef(out, "initializing ipfs node at %s\n", repoRoot); err != nil {
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

	if err := addDefaultAssets(out, repoRoot); err != nil {
		return err
	}

	return initializeIpnsKeyspace(repoRoot)
}

func addDefaultAssets(out io.Writer, repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := fsrepo.At(repoRoot)
	if err := r.Open(); err != nil { // NB: repo is owned by the node
		return err
	}
	nd, err := core.NewIPFSNode(ctx, core.Offline(r))
	if err != nil {
		return err
	}
	defer nd.Close()

	dirb := uio.NewDirectory(nd.DAG)

	// add every file in the assets pkg
	for fname, file := range assets.Init_dir {
		buf := bytes.NewBufferString(file)
		s, err := coreunix.Add(nd, buf)
		if err != nil {
			return err
		}
		k := u.B58KeyDecode(s)
		if err := dirb.AddChild(fname, k); err != nil {
			return err
		}
	}

	dir := dirb.GetNode()
	dkey, err := nd.DAG.Add(dir)
	if err != nil {
		return err
	}
	if err := nd.Pinning.Pin(dir, true); err != nil {
		return err
	}
	if err := nd.Pinning.Flush(); err != nil {
		return err
	}

	writef(out, "to get started, enter:\n")
	return writef(out, "\n\tipfs cat /ipfs/%s/readme\n\n", dkey)
}

func initializeIpnsKeyspace(repoRoot string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := fsrepo.At(repoRoot)
	if err := r.Open(); err != nil { // NB: repo is owned by the node
		return err
	}

	nd, err := core.NewIPFSNode(ctx, core.Offline(r))
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
