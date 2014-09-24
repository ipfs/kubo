package main

import (
	"errors"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/spf13/cobra"
)

var cmdIpfsMount = &cobra.Command{
	Use:   "mount",
	Short: "Mount an ipfs read-only mountpoint.",
	Long:  `Not yet implemented on windows.`,
	Run:   mountCmd,
}

func init() {
	CmdIpfs.AddCommand(cmdIpfsMount)
}

func mountCmd(c *cobra.Command, inp []string) {
	u.PErr(errors.New("mount not yet implemented on windows"))
}
