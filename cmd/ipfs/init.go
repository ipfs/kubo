package main

import (
	"encoding/base64"
	"os"

	"github.com/spf13/cobra"
	config "github.com/jbenet/go-ipfs/config"
	ci "github.com/jbenet/go-ipfs/crypto"
	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsInit = &cobra.Command{
	Use: "init",
	Short:     "Initialize ipfs local configuration",
	Long: `ipfs init

	Initializes ipfs configuration files and generates a
	new keypair.
`,
	Run:  initCmd,
}


var (
	bits int
	passphrase string
	force bool
)
func init() {
	cmdIpfsInit.Flags().IntVarP(&bits, "bits", "b", 4096, "number of bits for keypair")
	cmdIpfsInit.Flags().StringVarP(&passphrase, "passphrase", "p", "", "passphrase for encrypting keys")
	cmdIpfsInit.Flags().BoolVarP(&force, "force", "f", false, "force overwrite of existing config")
	CmdIpfs.AddCommand(cmdIpfsInit)
}

func initCmd(c *cobra.Command, inp []string) {
	configpath, err := getConfigDir(c)
	if err != nil {
		u.PErr(err.Error())
		return
	}

	u.POut("initializing ipfs node at %s\n", configpath)
	filename, err := config.Filename(configpath + "/config")
	if err != nil {
		u.PErr("Couldn't get home directory path")
		return
	}

	fi, err := os.Lstat(filename)
	if fi != nil || (err != nil && !os.IsNotExist(err)) {
		if !force {
			u.PErr("ipfs configuration file already exists!\nReinitializing would overwrite your keys.\n(use -f to force overwrite)")
			return
		}
	}
	cfg := new(config.Config)

	cfg.Datastore = config.Datastore{}
	dspath, err := u.TildeExpansion("~/.go-ipfs/datastore")
	if err != nil {
		u.PErr(err.Error())
		return
	}
	cfg.Datastore.Path = dspath
	cfg.Datastore.Type = "leveldb"

	cfg.Identity = new(config.Identity)
	// This needs thought
	cfg.Identity.Address = "/ip4/127.0.0.1/tcp/5001"

	// local RPC endpoint
	cfg.RPCAddress = "/ip4/127.0.0.1/tcp/4001"

	if bits < 1024 {
		u.PErr("Bitsize less than 1024 is considered unsafe.")
		return
	}

	u.POut("generating key pair\n")
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, bits)
	if err != nil {
		u.PErr(err.Error())
		return
	}

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		u.PErr(err.Error())
		return
	}
	cfg.Identity.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := spipe.IDFromPubKey(pk)
	if err != nil {
		u.PErr(err.Error())
		return
	}
	cfg.Identity.PeerID = id.Pretty()

	// Use these hardcoded bootstrap peers for now.
	cfg.Peers = []*config.SavedPeer{
		&config.SavedPeer{
			// mars.i.ipfs.io
			PeerID:  "QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
			Address: "/ip4/104.131.131.82/tcp/4001",
		},
	}

	path, err := u.TildeExpansion(config.DefaultConfigFilePath)
	if err != nil {
		u.PErr(err.Error())
		return
	}

	err = config.WriteConfigFile(path, cfg)
	if err != nil {
		u.PErr(err.Error())
		return
	}
}
