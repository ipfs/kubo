package main

import (
	"github.com/spf13/cobra"
	"github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
)

// The IPFS command tree. It is an instance of `cobra.Command`.
var CmdIpfs = &cobra.Command{
	Use: "ipfs [<flags>] <command> [<args>]",
	Short:     "global versioned p2p merkledag file system",
	Long: `ipfs - global versioned p2p merkledag file system
	Learn more at http://ipfs.io
	`,
}

func init() {
	CmdIpfs.PersistentFlags().StringP("config", "c", config.DefaultPathRoot, "config directory")
}

func main() {
	u.Debug = true
	CmdIpfs.Execute()
}

func localNode(confdir string, online bool) (*core.IpfsNode, error) {
	cfg, err := config.Load(confdir + "/config")
	if err != nil {
		return nil, err
	}

	return core.NewIpfsNode(cfg, online)
}

// Gets the config "-c" flag from the command, or returns
// the empty string
func getConfigDir(c *cobra.Command) (string, error) {
	conf := c.Flags().Lookup("c").Value.String()
	return conf, nil
}
