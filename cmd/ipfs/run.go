package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

var cmdIpfsRun = &commander.Command{
	UsageLine: "run",
	Short:     "run local ifps node.",
	Long: `run a local ipfs node with no other interface.
`,
	Run:  runCmd,
	Flag: *flag.NewFlagSet("ipfs-run", flag.ExitOnError),
}

func runCmd(c *commander.Command, inp []string) error {
	u.Debug = true

	n, err := localNode(true)
	if err != nil {
		return err
	}

	dl, err := daemon.NewDaemonListener(n, "localhost:12345")
	if err != nil {
		return err
	}
	dl.Listen()
	dl.Close()
	return nil
}
