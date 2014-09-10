package main

import (
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

func ex_make_cmd_subcmd2_cmd1() *commander.Command {
	cmd := &commander.Command{
		Run:       ex_run_cmd_subcmd2_cmd1,
		UsageLine: "cmd1 [options]",
		Short:     "runs cmd1 and exits",
		Long: `
runs cmd1 and exits.

ex:
 $ my-cmd subcmd2 cmd1
`,
		Flag: *flag.NewFlagSet("my-cmd-subcmd2-cmd1", flag.ExitOnError),
	}
	cmd.Flag.Bool("q", true, "only print error and warning messages, all other output will be suppressed")
	return cmd
}

func ex_run_cmd_subcmd2_cmd1(cmd *commander.Command, args []string) error {
	name := "my-cmd-subcmd2-" + cmd.Name()
	quiet := cmd.Flag.Lookup("q").Value.Get().(bool)
	fmt.Printf("%s: hello from subcmd2-cmd1 (quiet=%v)\n", name, quiet)
	return nil
}

// EOF
