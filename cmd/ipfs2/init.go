package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	imp "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	errors "github.com/jbenet/go-ipfs/util/debugerror"
)

var initCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Initializes IPFS config file",
		ShortDescription: "Initializes IPFS configuration files and generates a new keypair.",
	},

	Options: []cmds.Option{
		cmds.IntOption("bits", "b", "Number of bits to use in the generated RSA private key (defaults to 4096)"),
		cmds.StringOption("passphrase", "p", "Passphrase for encrypting the private key"),
		cmds.BoolOption("force", "f", "Overwrite existing config (if it exists)"),
		cmds.StringOption("datastore", "d", "Location for the IPFS data store"),

		// TODO need to decide whether to expose the override as a file or a
		// directory. That is: should we allow the user to also specify the
		// name of the file?
		// TODO cmds.StringOption("event-logs", "l", "Location for machine-readable event logs"),
	},
	Run: func(req cmds.Request) (interface{}, error) {

		dspathOverride, _, err := req.Option("d").String() // if !found it's okay. Let == ""
		if err != nil {
			return nil, err
		}

		force, _, err := req.Option("f").Bool() // if !found, it's okay force == false
		if err != nil {
			return nil, err
		}

		nBitsForKeypair, bitsOptFound, err := req.Option("b").Int()
		if err != nil {
			return nil, err
		}
		if !bitsOptFound {
			nBitsForKeypair = 4096
		}

		return doInit(req.Context().ConfigRoot, dspathOverride, force, nBitsForKeypair)
	},
}

var errCannotInitConfigExists = errors.New(`ipfs configuration file already exists!
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

For a short demo of what you can do, enter 'ipfs tour'
`

// NB: if dspath is not provided, it will be retrieved from the config
func doInit(configRoot string, dspathOverride string, force bool, nBitsForKeypair int) (interface{}, error) {

	u.POut("initializing ipfs node at %s\n", configRoot)

	configFilename, err := config.Filename(configRoot)
	if err != nil {
		return nil, errors.New("Couldn't get home directory path")
	}

	fi, err := os.Lstat(configFilename)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		if !force {
			// TODO multi-line string
			return nil, errCannotInitConfigExists
		}
	}

	conf, err := initConfig(configFilename, dspathOverride, nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	err = addTheWelcomeFile(conf, func(k u.Key) {
		fmt.Printf("\nto get started, enter: ipfs cat %s\n", k)
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// addTheWelcomeFile adds a file containing the welcome message to the newly
// minted node. On success, it calls onSuccess
func addTheWelcomeFile(conf *config.Config, onSuccess func(u.Key)) error {
	// TODO extract this file creation operation into a function
	nd, err := core.NewIpfsNode(conf, false)
	if err != nil {
		return err
	}
	defer nd.Close()

	// Set up default file
	reader := bytes.NewBufferString(welcomeMsg)

	defnd, err := imp.BuildDagFromReader(reader, nd.DAG, nd.Pinning.GetManual(), chunk.DefaultSplitter)
	if err != nil {
		return err
	}

	k, err := defnd.Key()
	if err != nil {
		return fmt.Errorf("failed to write test file: %s", err)
	}
	onSuccess(k)
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

	err := initCheckDir(dspath)
	if err != nil {
		return ds, errors.Errorf("datastore: %s", err)
	}

	return ds, nil
}

func initConfig(configFilename string, dspathOverride string, nBitsForKeypair int) (*config.Config, error) {
	ds, err := datastoreConfig(dspathOverride)
	if err != nil {
		return nil, err
	}

	identity, err := identityConfig(nBitsForKeypair, func() {
		fmt.Printf("generating key pair...")
	}, func(ident config.Identity) {
		fmt.Printf("done\n")
		fmt.Printf("peer identity: %s\n", ident.PeerID)
	})
	if err != nil {
		return nil, err
	}

	logConfig, err := initLogs("") // TODO allow user to override dir
	if err != nil {
		return nil, err
	}

	conf := &config.Config{

		// setup the node addresses.
		Addresses: config.Addresses{
			Swarm: "/ip4/0.0.0.0/tcp/4001",
			API:   "/ip4/127.0.0.1/tcp/5001",
		},

		Bootstrap: []*config.BootstrapPeer{
			&config.BootstrapPeer{ // Use these hardcoded bootstrap peers for now.
				// mars.i.ipfs.io
				PeerID:  "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
				Address: "/ip4/104.131.131.82/tcp/4001",
			},
		},

		Datastore: ds,

		Logs: logConfig,

		Identity: identity,

		// setup the node mount points.
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		// tracking ipfs version used to generate the init folder and adding
		// update checker default setting.
		Version: config.VersionDefaultValue(),
	}

	if err := config.WriteConfigFile(configFilename, conf); err != nil {
		return nil, err
	}

	return conf, nil
}

// identityConfig initializes a new identity. It calls onBegin when it begins
// to generate the identity and it calls onSuccess once the operation is
// completed successfully
func identityConfig(nbits int, onBegin func(), onSuccess func(config.Identity)) (config.Identity, error) {
	// TODO guard higher up
	ident := config.Identity{}
	if nbits < 1024 {
		return ident, errors.New("Bitsize less than 1024 is considered unsafe.")
	}

	onBegin()
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
	onSuccess(ident)
	return ident, nil
}

func initLogs(logpath string) (config.Logs, error) {
	if len(logpath) == 0 {
		var err error
		logpath, err = config.LogsPath("")
		if err != nil {
			return config.Logs{}, errors.Wrap(err)
		}
	}

	err := initCheckDir(logpath)
	if err != nil {
		return config.Logs{}, errors.Errorf("logs: %s", err)
	}

	return config.Logs{
		Filename: path.Join(logpath, "events.log"),
	}, nil
}

// initCheckDir ensures the directory exists and is writable
func initCheckDir(path string) error {
	// Construct the path if missing
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}

	// Check the directory is writeable
	if f, err := os.Create(filepath.Join(path, "._check_writeable")); err == nil {
		os.Remove(f.Name())
	} else {
		return errors.New("'" + path + "' is not writeable")
	}
	return nil
}
