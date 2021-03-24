package commands

import (
	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
)

var daemonShutdownCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Shut down the IPFS daemon.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsDaemon {
			return cmds.Errorf(cmds.ErrClient, "daemon not running")
		}

		if err := nd.Close(); err != nil {
			log.Error("error while shutting down ipfs daemon:", err)
		}

		return nil
	},
}
