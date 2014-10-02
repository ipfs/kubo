package main

import (
	"fmt"

	flag "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

var cmdIpfsName = &commander.Command{
	UsageLine: "name",
	Short:     "Ipfs namespace manipulation tools.",
	Long:      `ipfs name [publish|resolve] <ref/hash>`,
	Run:       addCmd,
	Flag:      *flag.NewFlagSet("ipfs-name", flag.ExitOnError),
	Subcommands: []*commander.Command{
		cmdIpfsPub,
		cmdIpfsResolve,
	},
}

func nameCmd(c *commander.Command, args []string) error {
	fmt.Println(c.Long)
	return nil
}
