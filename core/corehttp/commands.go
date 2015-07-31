package corehttp

import (
	"net"
	"net/http"
	"os"
	"strings"

	cors "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/rs/cors"

	commands "github.com/ipfs/go-ipfs/commands"
	cmdsHttp "github.com/ipfs/go-ipfs/commands/http"
	core "github.com/ipfs/go-ipfs/core"
	corecommands "github.com/ipfs/go-ipfs/core/commands"
	config "github.com/ipfs/go-ipfs/repo/config"
)

const originEnvKey = "API_ORIGIN"
const originEnvKeyDeprecate = `You are using the ` + originEnvKey + `ENV Variable.
This functionality is deprecated, and will be removed in future versions.
Instead, try either adding headers to the config, or passing them via
cli arguments:

	ipfs config API.HTTPHeaders 'Access-Control-Allow-Origin' '*'
	ipfs daemon

or

	ipfs daemon --api-http-header 'Access-Control-Allow-Origin: *'
`

func addCORSFromEnv(c *cmdsHttp.ServerConfig) {
	origin := os.Getenv(originEnvKey)
	if origin != "" {
		log.Warning(originEnvKeyDeprecate)
		if c.CORSOpts == nil {
			c.CORSOpts.AllowedOrigins = []string{origin}
		}
		c.CORSOpts.AllowedOrigins = append(c.CORSOpts.AllowedOrigins, origin)
	}
}

func addHeadersFromConfig(c *cmdsHttp.ServerConfig, nc *config.Config) {
	log.Info("Using API.HTTPHeaders:", nc.API.HTTPHeaders)

	if acao := nc.API.HTTPHeaders[cmdsHttp.ACAOrigin]; acao != nil {
		c.CORSOpts.AllowedOrigins = acao
	}
	if acam := nc.API.HTTPHeaders[cmdsHttp.ACAMethods]; acam != nil {
		c.CORSOpts.AllowedMethods = acam
	}
	if acac := nc.API.HTTPHeaders[cmdsHttp.ACACredentials]; acac != nil {
		for _, v := range acac {
			c.CORSOpts.AllowCredentials = (strings.ToLower(v) == "true")
		}
	}

	c.Headers = nc.API.HTTPHeaders
}

func CommandsOption(cctx commands.Context) ServeOption {
	return func(n *core.IpfsNode, l net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {

		cfg := &cmdsHttp.ServerConfig{
			CORSOpts: &cors.Options{
				AllowedMethods: []string{"GET", "POST", "PUT"},
			},
		}

		addHeadersFromConfig(cfg, n.Repo.Config())
		addCORSFromEnv(cfg)

		cmdHandler := cmdsHttp.NewHandler(cctx, corecommands.Root, cfg)
		mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
		return mux, nil
	}
}
