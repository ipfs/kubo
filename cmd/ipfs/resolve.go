package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsResolve = &commander.Command{
	UsageLine: "resolve",
	Short:     "resolve an ipns link to a hash",
	Long: `ipfs resolve <hash>... - Resolve hash.

`,
	Run:  resolveCmd,
	Flag: *flag.NewFlagSet("ipfs-resolve", flag.ExitOnError),
}

func resolveCmd(c *commander.Command, inp []string) error {
	u.Debug = true
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}
	conf, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}

	cmd := daemon.NewCommand()
	cmd.Command = "resolve"
	cmd.Args = inp
	err = daemon.SendCommand(cmd, conf)
	if err != nil {
		now := time.Now()
		// Resolve requires working DHT
		n, err := localNode(conf, true)
		if err != nil {
			return err
		}

		took := time.Now().Sub(now)
		fmt.Printf("localNode creation took %s\n", took.String())

		return commands.Resolve(n, cmd.Args, cmd.Opts, os.Stdout)
	}
	return nil
}
