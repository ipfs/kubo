package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsDiag = &commander.Command{
	UsageLine: "net-diag",
	Short:     "Generate a diagnostics report",
	Long: `ipfs net-diag - Generate a diagnostics report.

	Sends out a message to each node in the network recursively
	requesting a listing of data about them including number of
	connected peers and latencies between them.
`,
	Run:  diagCmd,
	Flag: *flag.NewFlagSet("ipfs-net-diag", flag.ExitOnError),
}

func init() {
	cmdIpfsDiag.Flag.Bool("raw", false, "print raw json output")
}

var diagCmd = makeCommand(command{
	name:  "diag",
	args:  0,
	flags: []string{"raw"},
	cmdFn: commands.Diag,
})
