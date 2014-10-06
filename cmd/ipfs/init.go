package main

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
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

	nbits, ok := c.Flag.Lookup("b").Value.Get().(int)
	if !ok {
		return errors.New("failed to get bits flag")
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

	id, err := spipe.IDFromPubKey(pk)
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
	cfg.Updates = config.Updates{
		Check:   "error",
		Version: currentVersion.String(),
	}

	err = config.WriteConfigFile(filename, cfg)
	if err != nil {
		return err
	}
	return nil
}
