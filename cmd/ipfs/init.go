package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	imp "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	peer "github.com/jbenet/go-ipfs/peer"
	updates "github.com/jbenet/go-ipfs/updates"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsInit = &commander.Command{
	UsageLine: "init",
	Short:     "Initialize ipfs local configuration",
	Long: `ipfs init

	Initializes ipfs configuration files and generates a
	new keypair.
`,
	Run:  initCmd,
	Flag: *flag.NewFlagSet("ipfs-init", flag.ExitOnError),
}

func init() {
	cmdIpfsInit.Flag.Int("b", 4096, "number of bits for keypair")
	cmdIpfsInit.Flag.String("p", "", "passphrase for encrypting keys")
	cmdIpfsInit.Flag.Bool("f", false, "force overwrite of existing config")
	cmdIpfsInit.Flag.String("d", "", "Change default datastore location")
}

var defaultPeers = []*config.BootstrapPeer{
	&config.BootstrapPeer{
		// mars.i.ipfs.io
		PeerID:  "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		Address: "/ip4/104.131.131.82/tcp/4001",
	},
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
	ident := config.Identity{}
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

func initCmd(c *commander.Command, inp []string) error {
	configpath, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}

	u.POut("initializing ipfs node at %s\n", configpath)
	filename, err := config.Filename(configpath)
	if err != nil {
		return errors.New("Couldn't get home directory path")
	}

	dspath, ok := c.Flag.Lookup("d").Value.Get().(string)
	if !ok {
		return errors.New("failed to parse datastore flag")
	}

	fi, err := os.Lstat(filename)
	force, ok := c.Flag.Lookup("f").Value.Get().(bool)
	if !ok {
		return errors.New("failed to parse force flag")
	}
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		if !force {
			return errors.New("ipfs configuration file already exists!\nReinitializing would overwrite your keys.\n(use -f to force overwrite)")
		}
	}
	cfg := new(config.Config)

	// setup the datastore
	cfg.Datastore, err = datastoreConfig(dspath)
	if err != nil {
		return err
	}

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

	nbits, ok := c.Flag.Lookup("b").Value.Get().(int)
	if !ok {
		return errors.New("failed to get bits flag")
	}
	if nbits < 1024 {
		return errors.New("Bitsize less than 1024 is considered unsafe.")
	}

	cfg.Identity, err = identityConfig(nbits)
	if err != nil {
		return err
	}

	// Use these hardcoded bootstrap peers for now.
	cfg.Bootstrap = defaultPeers

	// tracking ipfs version used to generate the init folder
	// and adding update checker default setting.
	cfg.Version = config.Version{
		Check:   "error",
		Current: updates.Version,
	}

	err = config.WriteConfigFile(filename, cfg)
	if err != nil {
		return err
	}

	nd, err := core.NewIpfsNode(cfg, false)
	if err != nil {
		return err
	}
	defer nd.Close()

	// Set up default file
	msg := `Hello and Welcome to IPFS!
If you're seeing this, that means that you have successfully
installed ipfs and are now interfacing with the wonderful
world of DAGs and hashes!
`
	reader := bytes.NewBufferString(msg)

	defnd, err := imp.BuildDagFromReader(reader, nd.DAG, nd.Pinning.GetManual(), chunk.DefaultSplitter)
	if err != nil {
		return err
	}

	k, _ := defnd.Key()
	fmt.Printf("Default file key: %s\n", k)

	return nil
}
