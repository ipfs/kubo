package main

import (
	"fmt"
	"os"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

var g_cmd = &commander.Command{
	UsageLine: os.Args[0] + " does cool things",
}

func init() {
	g_cmd.Subcommands = []*commander.Command{
		cmd_cmd1,
		ex_make_cmd_cmd2(),
		cmd_subcmd1,
		ex_make_cmd_subcmd2(),
	}
}

func main() {
	err := g_cmd.Dispatch(os.Args[1:])
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	return
}

// EOF
