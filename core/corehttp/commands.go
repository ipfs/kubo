package corehttp

import (
	"net/http"
	"os"

	commands "github.com/jbenet/go-ipfs/thirdparty/commands"
	cmdsHttp "github.com/jbenet/go-ipfs/thirdparty/commands/http"
	core "github.com/jbenet/go-ipfs/core"
	corecommands "github.com/jbenet/go-ipfs/core/commands"
)

const (
	// TODO rename
	originEnvKey = "API_ORIGIN"
)

func CommandsOption(cctx commands.Context) ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) error {
		origin := os.Getenv(originEnvKey)
		cmdHandler := cmdsHttp.NewHandler(cctx, corecommands.Root, origin)
		mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
		return nil
	}
}
