package main

import (
	"bytes"
	"fmt"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	assets "github.com/jbenet/go-ipfs/assets"
	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	coreunix "github.com/jbenet/go-ipfs/core/coreunix"
	ipns "github.com/jbenet/go-ipfs/fuse/ipns"
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

		output, err := doInit(req.Context().ConfigRoot, force, nBitsForKeypair)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
}

var errRepoExists = debugerror.New(`ipfs configuration file already exists!
Reinitializing would overwrite your keys.
(use -f to force overwrite)
`)

var welcomeMsg = `Hello and Welcome to IPFS!

██╗██████╗ ███████╗███████╗
██║██╔══██╗██╔════╝██╔════╝
██║██████╔╝█████╗  ███████╗
██║██╔═══╝ ██╔══╝  ╚════██║
██║██║     ██║     ███████║
╚═╝╚═╝     ╚═╝     ╚══════╝

If you're seeing this, you have successfully installed
IPFS and are now interfacing with the ipfs merkledag!

`

func initWithDefaults(repoRoot string) error {
	_, err := doInit(repoRoot, false, nBitsForKeypairDefault)
	return debugerror.Wrap(err)
}

func doInit(repoRoot string, force bool, nBitsForKeypair int) (interface{}, error) {
	u.POut("initializing ipfs node at %s\n", repoRoot)

	if fsrepo.IsInitialized(repoRoot) && !force {
		return nil, errRepoExists
	}
	conf, err := config.Init(nBitsForKeypair)
	if err != nil {
		return nil, err
	}
	if fsrepo.IsInitialized(repoRoot) {
		if err := fsrepo.Remove(repoRoot); err != nil {
			return nil, err
		}
	}
	if err := fsrepo.Init(repoRoot, conf); err != nil {
		return nil, err
	}

	if err := addDefaultAssets(repoRoot); err != nil {
		return nil, err
	}

	err = initializeIpnsKeyspace(repoRoot)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func addDefaultAssets(repoRoot string) error {
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

	contact := bytes.NewBufferString(assets.Init_doc_contact)
	contact_key, err := coreunix.Add(nd, contact)
	if err != nil {
		return err
	}
	err = dirb.AddChild("contact", contact_key)
	if err != nil {
		return err
	}

	help := bytes.NewBufferString(assets.Init_doc_help)
	help_key, err := coreunix.Add(nd, help)
	if err != nil {
		return err
	}
	err = dirb.AddChild("help", help_key)
	if err != nil {
		return err
	}

	readme := bytes.NewBufferString(assets.Init_doc_readme)
	readme_key, err := coreunix.Add(nd, readme)
	if err != nil {
		return err
	}
	err = dirb.AddChild("readme", readme_key)
	if err != nil {
		return err
	}

	secnotes := bytes.NewBufferString(assets.Init_doc_security_notes)
	secnotes_key, err := coreunix.Add(nd, secnotes)
	if err != nil {
		return err
	}
	err = dirb.AddChild("security-notes", secnotes_key)
	if err != nil {
		return err
	}

	dir := dirb.GetNode()
	dkey, err := nd.DAG.Add(dir)
	if err != nil {
		return err
	}

	fmt.Printf("More documentation in %s\n", dkey)
	return nil
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

	return ipns.InitializeKeyspace(nd, nd.PrivateKey)
}
