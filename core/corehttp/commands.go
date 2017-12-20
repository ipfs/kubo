package corehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	corecommands "github.com/ipfs/go-ipfs/core/commands"
	path "github.com/ipfs/go-ipfs/path"
	config "github.com/ipfs/go-ipfs/repo/config"

	cmds "gx/ipfs/QmUjA7onJjYmZ2TivD83MeCfXf5o1U4ByVJWPjRru5NA4t/go-ipfs-cmds"
	cmdsHttp "gx/ipfs/QmUjA7onJjYmZ2TivD83MeCfXf5o1U4ByVJWPjRru5NA4t/go-ipfs-cmds/http"
)

var (
	errApiVersionMismatch = errors.New("api version mismatch")
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

const APIPath = "/api/v0"

var defaultLocalhostOrigins = []string{
	"http://127.0.0.1:<port>",
	"https://127.0.0.1:<port>",
	"http://localhost:<port>",
	"https://localhost:<port>",
}

func addCORSFromEnv(c *cmdsHttp.ServerConfig) {
	origin := os.Getenv(originEnvKey)
	if origin != "" {
		log.Warning(originEnvKeyDeprecate)
		if len(c.AllowedOrigins()) == 0 {
			c.SetAllowedOrigins([]string{origin}...)
		}
		c.AppendAllowedOrigins(origin)
	}
}

func addHeadersFromConfig(c *cmdsHttp.ServerConfig, nc *config.Config) {
	log.Info("Using API.HTTPHeaders:", nc.API.HTTPHeaders)

	if acao := nc.API.HTTPHeaders[cmdsHttp.ACAOrigin]; acao != nil {
		c.SetAllowedOrigins(acao...)
	}
	if acam := nc.API.HTTPHeaders[cmdsHttp.ACAMethods]; acam != nil {
		c.SetAllowedMethods(acam...)
	}
	if acac := nc.API.HTTPHeaders[cmdsHttp.ACACredentials]; acac != nil {
		for _, v := range acac {
			c.SetAllowCredentials(strings.ToLower(v) == "true")
		}
	}

	c.Headers = nc.API.HTTPHeaders
}

func addCORSDefaults(c *cmdsHttp.ServerConfig) {
	// by default use localhost origins
	if len(c.AllowedOrigins()) == 0 {
		c.SetAllowedOrigins(defaultLocalhostOrigins...)
	}

	// by default, use GET, PUT, POST
	if len(c.AllowedMethods()) == 0 {
		c.SetAllowedMethods("GET", "POST", "PUT")
	}
}

func patchCORSVars(c *cmdsHttp.ServerConfig, addr net.Addr) {

	// we have to grab the port from an addr, which may be an ip6 addr.
	// TODO: this should take multiaddrs and derive port from there.
	port := ""
	if tcpaddr, ok := addr.(*net.TCPAddr); ok {
		port = strconv.Itoa(tcpaddr.Port)
	} else if udpaddr, ok := addr.(*net.UDPAddr); ok {
		port = strconv.Itoa(udpaddr.Port)
	}

	// we're listening on tcp/udp with ports. ("udp!?" you say? yeah... it happens...)
	origins := c.AllowedOrigins()
	for i, o := range origins {
		// TODO: allow replacing <host>. tricky, ip4 and ip6 and hostnames...
		if port != "" {
			o = strings.Replace(o, "<port>", port, -1)
		}
		origins[i] = o
	}
	c.SetAllowedOrigins(origins...)
}

func commandsOption(cctx oldcmds.Context, command *cmds.Command) ServeOption {
	return func(n *core.IpfsNode, l net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {

		cfg := cmdsHttp.NewServerConfig()
		cfg.SetAllowedMethods("GET", "POST", "PUT")
		cfg.APIPath = APIPath
		rcfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		addHeadersFromConfig(cfg, rcfg)
		addCORSFromEnv(cfg)
		addCORSDefaults(cfg)
		patchCORSVars(cfg, l.Addr())

		cmdHandler := cmdsHttp.NewHandler(&cctx, command, cfg)
		mux.Handle(APIPath+"/", cmdHandler)
		return mux, nil
	}
}

func CommandsOption(cctx oldcmds.Context) ServeOption {
	return commandsOption(cctx, corecommands.Root)
}

func CommandsROOption(cctx oldcmds.Context) ServeOption {
	return commandsOption(cctx, corecommands.RootRO)
}

// CheckVersionOption returns a ServeOption that checks whether the client ipfs version matches. Does nothing when the user agent string does not contain `/go-ipfs/`
func CheckVersionOption() ServeOption {
	daemonVersion := config.ApiVersion

	return ServeOption(func(n *core.IpfsNode, l net.Listener, parent *http.ServeMux) (*http.ServeMux, error) {
		mux := http.NewServeMux()
		parent.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, APIPath) {
				cmdqry := r.URL.Path[len(APIPath):]
				pth := path.SplitList(cmdqry)

				// backwards compatibility to previous version check
				if pth[1] != "version" {
					clientVersion := r.UserAgent()
					// skips check if client is not go-ipfs
					if clientVersion != "" && strings.Contains(clientVersion, "/go-ipfs/") && daemonVersion != clientVersion {
						http.Error(w, fmt.Sprintf("%s (%s != %s)", errApiVersionMismatch, daemonVersion, clientVersion), http.StatusBadRequest)
						return
					}
				}
			}

			mux.ServeHTTP(w, r)
		})

		return mux, nil
	})
}
