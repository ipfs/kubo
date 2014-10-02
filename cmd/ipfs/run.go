package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
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
	cc, err := setupCmdContext(c, true)
	if err != nil {
		return err
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP, syscall.SIGINT,
		syscall.SIGTERM, syscall.SIGQUIT)

	// wait until we get a signal to exit.
	<-sigc

	cc.daemon.Close()
	return nil
}
