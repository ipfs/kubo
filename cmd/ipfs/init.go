package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"path/filepath"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	corecmds "github.com/jbenet/go-ipfs/core/commands"
	imp "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	ci "github.com/jbenet/go-ipfs/p2p/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	repo "github.com/jbenet/go-ipfs/repo"
	config "github.com/jbenet/go-ipfs/repo/config"
	"github.com/jbenet/go-ipfs/repo/fsrepo"
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
	Run: func(req cmds.Request) (interface{}, error) {

		force, _, err := req.Option("f").Bool() // if !found, it's okay force == false
		if err != nil {
			return nil, err
		}

		nBitsForKeypair, bitsOptFound, err := req.Option("b").Int()
		if err != nil {
			return nil, err
		}
		if !bitsOptFound {
			nBitsForKeypair = nBitsForKeypairDefault
		}

		return doInit(req.Context().ConfigRoot, force, nBitsForKeypair)
	},
}

var errCannotInitConfigExists = debugerror.New(`ipfs configuration file already exists!
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

func initWithDefaults(configRoot string) error {
	_, err := doInit(configRoot, false, nBitsForKeypairDefault)
	return debugerror.Wrap(err)
}

func doInit(configRoot string, force bool, nBitsForKeypair int) (interface{}, error) {

	u.POut("initializing ipfs node at %s\n", configRoot)

	if fsrepo.ConfigIsInitialized(configRoot) && !force {
		return nil, errCannotInitConfigExists
	}

	conf, err := initConfig(nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	repo := fsrepo.At(configRoot)
	if err := repo.Open(); err != nil {
		return nil, err
	}
	if err := repo.SetConfig(conf); err != nil {
		return nil, err
	}
	if err := repo.Close(); err != nil {
		return nil, err
	}

	err = addTheWelcomeFile(conf)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// addTheWelcomeFile adds a file containing the welcome message to the newly
// minted node. On success, it calls onSuccess
func addTheWelcomeFile(conf *config.Config) error {
	// TODO extract this file creation operation into a function
	ctx, cancel := context.WithCancel(context.Background())
	nd, err := core.NewIPFSNode(ctx, core.Offline(conf))
	if err != nil {
		return err
	}
	defer nd.Close()
	defer cancel()

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
	fmt.Printf("\nto get started, enter: ipfs cat %s\n", k)
	return nil
}

func datastoreConfig() (config.Datastore, error) {
	ds := config.Datastore{}
	dspath, err := config.DataStorePath("")
	if err != nil {
		return ds, err
	}
	ds.Path = dspath
	ds.Type = "leveldb"
	if err := initCheckDir(dspath); err != nil {
		return ds, debugerror.Errorf("datastore: %s", err)
	}
	return ds, nil
}

func initConfig(nBitsForKeypair int) (*config.Config, error) {
	ds, err := datastoreConfig()
	if err != nil {
		return nil, err
	}

	identity, err := identityConfig(nBitsForKeypair)
	if err != nil {
		return nil, err
	}

	logConfig, err := initLogs("") // TODO allow user to override dir
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := corecmds.DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	conf := &config.Config{

		// setup the node's default addresses.
		// Note: two swarm listen addrs, one tcp, one utp.
		Addresses: config.Addresses{
			Swarm: []string{
				"/ip4/0.0.0.0/tcp/4001",
				// "/ip4/0.0.0.0/udp/4002/utp", // disabled for now.
			},
			API: "/ip4/127.0.0.1/tcp/5001",
		},

		Bootstrap: bootstrapPeers,
		Datastore: ds,
		Logs:      logConfig,
		Identity:  identity,

		// setup the node mount points.
		Mounts: config.Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		// tracking ipfs version used to generate the init folder and adding
		// update checker default setting.
		Version: config.VersionDefaultValue(),
	}

	return conf, nil
}

// identityConfig initializes a new identity.
func identityConfig(nbits int) (config.Identity, error) {
	// TODO guard higher up
	ident := config.Identity{}
	if nbits < 1024 {
		return ident, debugerror.New("Bitsize less than 1024 is considered unsafe.")
	}

	fmt.Printf("generating key pair...")
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return ident, err
	}
	fmt.Printf("done\n")

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()
	fmt.Printf("peer identity: %s\n", ident.PeerID)
	return ident, nil
}

// initLogs initializes the event logger at the specified path. It uses the
// default log path if no path is provided.
func initLogs(logpath string) (config.Logs, error) {
	if len(logpath) == 0 {
		var err error
		logpath, err = config.LogsPath("")
		if err != nil {
			return config.Logs{}, debugerror.Wrap(err)
		}
	}
	err := initCheckDir(logpath)
	if err != nil {
		return config.Logs{}, debugerror.Errorf("logs: %s", err)
	}
	conf := config.Logs{
		Filename: path.Join(logpath, "events.log"),
	}
	err = repo.ConfigureEventLogger(conf)
	if err != nil {
		return conf, err
	}
	return conf, nil
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
		return debugerror.New("'" + path + "' is not writeable")
	}
	return nil
}
