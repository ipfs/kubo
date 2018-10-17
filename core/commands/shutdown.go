package commands

import (
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	"github.com/ipfs/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmVy9gWXWJB8GrQG85Sq7hCknC6ANqZjJCZkRo8Y6sk5tx/go-ipfs-cmds"
)

var daemonShutdownCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Shut down the ipfs daemon",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if nd.LocalMode() {
			return cmdkit.Errorf(cmdkit.ErrClient, "daemon not running")
		}

		if err := nd.Process().Close(); err != nil {
			log.Error("error while shutting down ipfs daemon:", err)
		}

		return nil
	},
}
