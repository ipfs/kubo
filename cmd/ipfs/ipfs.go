package main

import (
	"github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/spf13/cobra"
)

// The IPFS command tree. It is an instance of `cobra.Command`.
var CmdIpfs = &cobra.Command{
	Use:   "ipfs [<flags>] <command> [<args>]",
	Short: "global versioned p2p merkledag file system",
	Long: `ipfs - global versioned p2p merkledag file system
Learn more at http://ipfs.io
`,
}

var configDir string

func init() {
	CmdIpfs.PersistentFlags().StringVarP(&configDir, "config", "c", config.DefaultPathRoot, "config directory")
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
	if configDir == "" {
		return u.TildeExpansion("~/.go-ipfs")
	}
	return u.TildeExpansion(configDir)
}
