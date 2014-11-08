package main

import (
	"encoding/base64"
	"errors"
	"fmt"
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
	Description: "Initializes IPFS config file",
	Help: `Initializes IPFS configuration files and generates a new keypair.
`,

	Options: []cmds.Option{
		cmds.Option{[]string{"bits", "b"}, cmds.Int,
			"Number of bits to use in the generated RSA private key (defaults to 4096)"},
		cmds.Option{[]string{"passphrase", "p"}, cmds.String,
			"Passphrase for encrypting the private key"},
		cmds.Option{[]string{"force", "f"}, cmds.Bool,
			"Overwrite existing config (if it exists)"},
		cmds.Option{[]string{"datastore", "d"}, cmds.String,
			"Location for the IPFS data store"},
	},
	Run: func(res cmds.Response, req cmds.Request) {

		arg, found := req.Option("d")
		dspath, ok := arg.(string)
		if found && !ok {
			res.SetError(errors.New("failed to parse datastore flag"), cmds.ErrNormal)
			return
		}

		arg, found = req.Option("f")
		force, ok := arg.(bool) // TODO param
		if found && !ok {
			res.SetError(errors.New("failed to parse force flag"), cmds.ErrNormal)
			return
		}

		arg, found = req.Option("b")
		nBitsForKeypair, ok := arg.(int) // TODO param
		if found && !ok {
			res.SetError(errors.New("failed to get bits flag"), cmds.ErrNormal)
			return
		} else if !found {
			nBitsForKeypair = 4096
		}

		err := doInit(req.Context().ConfigRoot, dspath, force, nBitsForKeypair)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

// Use these hardcoded bootstrap peers for now.
var defaultPeers = []*config.BootstrapPeer{
	&config.BootstrapPeer{
		// mars.i.ipfs.io
		PeerID:  "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		Address: "/ip4/104.131.131.82/tcp/4001",
	},
}

// TODO add default welcome hash: eaa68bedae247ed1e5bd0eb4385a3c0959b976e4
func doInit(configRoot string, dspath string, force bool, nBitsForKeypair int) error {

	u.POut("initializing ipfs node at %s\n", configRoot)

	configFilename, err := config.Filename(configRoot)
	if err != nil {
		return errors.New("Couldn't get home directory path")
	}

	fi, err := os.Lstat(configFilename)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		if !force {
			// TODO multi-line string
			return errors.New("ipfs configuration file already exists!\nReinitializing would overwrite your keys.\n(use -f to force overwrite)")
		}
	}

	ds, err := datastoreConfig(dspath)
	if err != nil {
		return err
	}

	identity, err := identityConfig(nBitsForKeypair)
	if err != nil {
		return err
	}

	conf := config.Config{

		// setup the node addresses.
		Addresses: config.Addresses{
			Swarm: "/ip4/0.0.0.0/tcp/4001",
			API:   "/ip4/127.0.0.1/tcp/5001",
		},

		Bootstrap: defaultPeers,

		Datastore: ds,

		Identity: identity,

		// setup the node mount points.
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		// tracking ipfs version used to generate the init folder and adding
		// update checker default setting.

		// FIXME(brian): before merging into master, change this to...
		// Version: config.VersionDefaultValue()
		Version: config.Version{
			Check:   "error",
			Current: updates.Version,
		},
	}

	err = config.WriteConfigFile(configFilename, conf)
	if err != nil {
		return err
	}
	return nil
}

func datastoreConfig(dspath string) (config.Datastore, error) {
	ds := config.Datastore{}
	if len(dspath) == 0 {
		var err error
		dspath, err = config.DataStorePath("")
		if err != nil {
			return ds, err
		}
	}
	ds.Path = dspath
	ds.Type = "leveldb"

	// Construct the data store if missing
	if err := os.MkdirAll(dspath, os.ModePerm); err != nil {
		return ds, err
	}

	// Check the directory is writeable
	if f, err := os.Create(filepath.Join(dspath, "._check_writeable")); err == nil {
		os.Remove(f.Name())
	} else {
		return ds, errors.New("Datastore '" + dspath + "' is not writeable")
	}

	return ds, nil
}

func identityConfig(nbits int) (config.Identity, error) {
	// TODO guard higher up
	ident := config.Identity{}
	if nbits < 1024 {
		return ident, errors.New("Bitsize less than 1024 is considered unsafe.")
	}

	fmt.Println("generating key pair...")
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return ident, err
	}

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPubKey(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()

	return ident, nil
}
