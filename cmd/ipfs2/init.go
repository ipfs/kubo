package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	updates "github.com/jbenet/go-ipfs/updates"
	u "github.com/jbenet/go-ipfs/util"
)

var initCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"bits", "b"}, cmds.Int},
		cmds.Option{[]string{"passphrase", "p"}, cmds.String},
		cmds.Option{[]string{"force", "f"}, cmds.Bool},
		cmds.Option{[]string{"datastore", "d"}, cmds.String},
	},
	Help: `ipfs init

	Initializes ipfs configuration files and generates a
	new keypair.
	`,
	Run: func(res cmds.Response, req cmds.Request) {

		arg, found := req.Option("d")
		dspath, ok := arg.(string) // TODO param
		if found && !ok {
			res.SetError(errors.New("failed to parse datastore flag"), cmds.ErrNormal)
			return
		}

		err := foo(res, req, dspath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

func foo(res cmds.Response, req cmds.Request, dspath string) error {
	ctx := req.Context()

	u.POut("initializing ipfs node at %s\n", ctx.ConfigRoot)
	filename, err := config.Filename(ctx.ConfigRoot)
	if err != nil {
		return errors.New("Couldn't get home directory path")
	}

	arg, found := req.Option("f")
	force, ok := arg.(bool) // TODO param
	if found && !ok {
		return errors.New("failed to parse force flag")
	}

	fi, err := os.Lstat(filename)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		if !force {
			// TODO multi-line string
			return errors.New("ipfs configuration file already exists!\nReinitializing would overwrite your keys.\n(use -f to force overwrite)")
		}
	}
	cfg := new(config.Config)

	cfg.Datastore = config.Datastore{}
	if len(dspath) == 0 {
		dspath, err = config.DataStorePath("")
		if err != nil {
			return err
		}
	}
	cfg.Datastore.Path = dspath
	cfg.Datastore.Type = "leveldb"

	// Construct the data store if missing
	if err := os.MkdirAll(dspath, os.ModePerm); err != nil {
		return err
	}

	// Check the directory is writeable
	if f, err := os.Create(filepath.Join(dspath, "._check_writeable")); err == nil {
		os.Remove(f.Name())
	} else {
		return errors.New("Datastore '" + dspath + "' is not writeable")
	}

	cfg.Identity = config.Identity{}

	// setup the node addresses.
	cfg.Addresses = config.Addresses{
		Swarm: "/ip4/0.0.0.0/tcp/4001",
		API:   "/ip4/127.0.0.1/tcp/5001",
	}

	// setup the node mount points.
	cfg.Mounts = config.Mounts{
		IPFS: "/ipfs",
		IPNS: "/ipns",
	}

	arg, found = req.Option("b")
	nbits, ok := arg.(int) // TODO param
	if found && !ok {
		return errors.New("failed to get bits flag")
	} else if !found {
		nbits = 4096
	}
	if nbits < 1024 {
		return errors.New("Bitsize less than 1024 is considered unsafe.")
	}

	u.POut("generating key pair\n")
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return err
	}

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return err
	}
	cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPubKey(pk)
	if err != nil {
		return err
	}
	cfg.Identity.PeerID = id.Pretty()

	// Use these hardcoded bootstrap peers for now.
	cfg.Bootstrap = []*config.BootstrapPeer{
		&config.BootstrapPeer{
			// mars.i.ipfs.io
			PeerID:  "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
			Address: "/ip4/104.131.131.82/tcp/4001",
		},
	}

	// tracking ipfs version used to generate the init folder and adding update checker default setting.
	cfg.Version = config.Version{
		Check:   "error",
		Current: updates.Version,
	}

	err = config.WriteConfigFile(filename, cfg)
	if err != nil {
		return err
	}
	return nil
}
