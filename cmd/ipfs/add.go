package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
	"github.com/jbenet/go-ipfs/daemon"
	u "github.com/jbenet/go-ipfs/util"
)

// Error indicating the max depth has been exceded.
var ErrDepthLimitExceeded = fmt.Errorf("depth limit exceeded")

var cmdIpfsAdd = &commander.Command{
	UsageLine: "add",
	Short:     "Add an object to ipfs.",
	Long: `ipfs add <path>... - Add objects to ipfs.

    Adds contents of <path> to ipfs. Use -r to add directories.
    Note that directories are added recursively, to form the ipfs
    MerkleDAG. A smarter partial add with a staging area (like git)
    remains to be implemented.
`,
	Run:  addCmd,
	Flag: *flag.NewFlagSet("ipfs-add", flag.ExitOnError),
}

func init() {
	cmdIpfsAdd.Flag.Bool("r", false, "add objects recursively")
}

func addCmd(c *commander.Command, inp []string) error {
	if len(inp) < 1 {
		u.POut(c.Long)
		return nil
	}
	conf, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}

	for i, fi := range inp {
		abspath, err := filepath.Abs(fi)
		if err != nil {
			return err
		}
		inp[i] = abspath
	}

	cmd := daemon.NewCommand()
	cmd.Command = "add"
	cmd.Args = inp
	cmd.Opts["r"] = c.Flag.Lookup("r").Value.Get()
	err = daemon.SendCommand(cmd, conf)
	if err != nil {
		fmt.Println(err)
		// Do locally

		now := time.Now()
		n, err := localNode(conf, false)
		if err != nil {
			return err
		}

		took := time.Now().Sub(now)
		fmt.Printf("localNode creation took %s\n", took.String())

		return commands.Add(n, cmd.Args, cmd.Opts, os.Stdout)
	}
	return nil
}
