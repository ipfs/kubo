package main

import (
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsPub = &commander.Command{
	UsageLine: "publish",
	Short:     "Publish an object to ipns under your key.",
	Long: `ipfs publish <path> - Publish object to ipns.

`,
	Run:  pubCmd,
	Flag: *flag.NewFlagSet("ipfs-publish", flag.ExitOnError),
}

func init() {
	cmdIpfsPub.Flag.String("k", "", "Specify key to use for publishing.")
}

func pubCmd(c *commander.Command, inp []string) error {
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
	cmd.Command = "publish"
	cmd.Args = inp
	cmd.Opts["k"] = c.Flag.Lookup("k").Value.Get()
	err = daemon.SendCommand(cmd, conf)
	if err != nil {
		u.DOut("Executing command locally.\n")
		// Do locally
		conf, err := getConfigDir(c.Parent)
		if err != nil {
			return err
		}
		n, err := localNode(conf, true)
		if err != nil {
			return err
		}

		return commands.Publish(n, cmd.Args, cmd.Opts, os.Stdout)
	}
	return nil
}
