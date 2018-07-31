package commands

import (
	"fmt"

	cmds "gx/ipfs/QmNueRyPRQiV7PUEpnP4GgGLuK1rKQLaRW7sfPvUetYig1/go-ipfs-cmds"
	"gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"
)

var daemonShutdownCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Shut down the ipfs daemon",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) {
		nd, err := GetNode(env)
		if err != nil {
			re.SetError(err, cmdkit.ErrNormal)
			return
		}

		if nd.LocalMode() {
			re.SetError(fmt.Errorf("daemon not running"), cmdkit.ErrClient)
			return
		}

		if err := nd.Process().Close(); err != nil {
			log.Error("error while shutting down ipfs daemon:", err)
		}
	},
}
