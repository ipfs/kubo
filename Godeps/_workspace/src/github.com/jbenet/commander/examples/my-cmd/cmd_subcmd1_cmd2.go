package main

import (
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

var cmd_subcmd1_cmd2 = &commander.Command{
	Run:       ex_run_cmd_subcmd1_cmd2,
	UsageLine: "cmd2 [options]",
	Short:     "runs cmd2 and exits",
	Long: `
runs cmd2 and exits.

ex:
$ my-cmd subcmd1 cmd2
`,
	Flag: *flag.NewFlagSet("my-cmd-subcmd1-cmd2", flag.ExitOnError),
}

func init() {
	cmd_subcmd1_cmd2.Flag.Bool("q", true, "only print error and warning messages, all other output will be suppressed")
}

func ex_run_cmd_subcmd1_cmd2(cmd *commander.Command, args []string) error {
	name := "my-cmd-subcmd1-" + cmd.Name()
	quiet := cmd.Flag.Lookup("q").Value.Get().(bool)
	fmt.Printf("%s: hello from subcmd1-cmd2 (quiet=%v)\n", name, quiet)
	return nil
}

// EOF
