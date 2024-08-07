package corehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	cmdsHttp "github.com/ipfs/go-ipfs-cmds/http"
	version "github.com/ipfs/kubo"
	oldcmds "github.com/ipfs/kubo/commands"
	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	corecommands "github.com/ipfs/kubo/core/commands"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var errAPIVersionMismatch = errors.New("api version mismatch")

const (
	originEnvKey          = "API_ORIGIN"
	originEnvKeyDeprecate = `You are using the ` + originEnvKey + `ENV Variable.
This functionality is deprecated, and will be removed in future versions.
Instead, try either adding headers to the config, or passing them via
cli arguments:

	ipfs config API.HTTPHeaders --json '{"Access-Control-Allow-Origin": ["*"]}'
	ipfs daemon
`
)

// APIPath is the path at which the API is mounted.
const APIPath = "/api/v0"

var defaultLocalhostOrigins = []string{
	"http://127.0.0.1:<port>",
	"https://127.0.0.1:<port>",
	"http://[::1]:<port>",
	"https://[::1]:<port>",
	"http://localhost:<port>",
	"https://localhost:<port>",
}

var companionBrowserExtensionOrigins = []string{
	"chrome-extension://nibjojkomfdiaoajekhjakgkdhaomnch", // ipfs-companion
	"chrome-extension://hjoieblefckbooibpepigmacodalfndh", // ipfs-companion-beta
}

func addCORSFromEnv(c *cmdsHttp.ServerConfig) {
	origin := os.Getenv(originEnvKey)
	if origin != "" {
		log.Warn(originEnvKeyDeprecate)
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
	for _, v := range nc.API.HTTPHeaders[cmdsHttp.ACACredentials] {
		c.SetAllowCredentials(strings.ToLower(v) == "true")
	}

	c.Headers = make(map[string][]string, len(nc.API.HTTPHeaders)+1)

	// Copy these because the config is shared and this function is called
	// in multiple places concurrently. Updating these in-place *is* racy.
	for h, v := range nc.API.HTTPHeaders {
		h = http.CanonicalHeaderKey(h)
		switch h {
		case cmdsHttp.ACAOrigin, cmdsHttp.ACAMethods, cmdsHttp.ACACredentials:
			// these are handled by the CORs library.
		default:
			c.Headers[h] = v
		}
	}
	c.Headers["Server"] = []string{"kubo/" + version.CurrentVersionNumber}
}

func addCORSDefaults(c *cmdsHttp.ServerConfig) {
	// always safelist certain origins
	c.AppendAllowedOrigins(defaultLocalhostOrigins...)
	c.AppendAllowedOrigins(companionBrowserExtensionOrigins...)

	// by default, use GET, PUT, POST
	if len(c.AllowedMethods()) == 0 {
		c.SetAllowedMethods(http.MethodGet, http.MethodPost, http.MethodPut)
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
	oldOrigins := c.AllowedOrigins()
	newOrigins := make([]string, len(oldOrigins))
	for i, o := range oldOrigins {
		// TODO: allow replacing <host>. tricky, ip4 and ip6 and hostnames...
		if port != "" {
			o = strings.Replace(o, "<port>", port, -1)
		}
		newOrigins[i] = o
	}
	c.SetAllowedOrigins(newOrigins...)
}

func commandsOption(cctx oldcmds.Context, command *cmds.Command) ServeOption {
	return func(n *core.IpfsNode, l net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		cfg := cmdsHttp.NewServerConfig()

		cfg.AddAllowedHeaders("Origin", "Accept", "Content-Type", "X-Requested-With")
		cfg.SetAllowedMethods(http.MethodPost)

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

		if len(rcfg.API.Authorizations) > 0 {
			authorizations := convertAuthorizationsMap(rcfg.API.Authorizations)
			cmdHandler = withAuthSecrets(authorizations, cmdHandler)
		}

		cmdHandler = otelhttp.NewHandler(cmdHandler, "corehttp.cmdsHandler")
		mux.Handle(APIPath+"/", cmdHandler)
		return mux, nil
	}
}

type rpcAuthScopeWithUser struct {
	config.RPCAuthScope
	User string
}

func convertAuthorizationsMap(authScopes map[string]*config.RPCAuthScope) map[string]rpcAuthScopeWithUser {
	// authorizations is a map where we can just check for the header value to match.
	authorizations := map[string]rpcAuthScopeWithUser{}
	for user, authScope := range authScopes {
		expectedHeader := config.ConvertAuthSecret(authScope.AuthSecret)
		if expectedHeader != "" {
			authorizations[expectedHeader] = rpcAuthScopeWithUser{
				RPCAuthScope: *authScopes[user],
				User:         user,
			}
		}
	}

	return authorizations
}

func withAuthSecrets(authorizations map[string]rpcAuthScopeWithUser, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorizationHeader := r.Header.Get("Authorization")
		auth, ok := authorizations[authorizationHeader]

		if ok {
			// version check is implicitly allowed
			if r.URL.Path == "/api/v0/version" {
				next.ServeHTTP(w, r)
				return
			}
			// everything else has to be safelisted via AllowedPaths
			for _, prefix := range auth.AllowedPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		http.Error(w, "Kubo RPC Access Denied: Please provide a valid authorization token as defined in the API.Authorizations configuration.", http.StatusForbidden)
	})
}

// CommandsOption constructs a ServerOption for hooking the commands into the
// HTTP server. It will NOT allow GET requests.
func CommandsOption(cctx oldcmds.Context) ServeOption {
	return commandsOption(cctx, corecommands.Root)
}

// CheckVersionOption returns a ServeOption that checks whether the client ipfs version matches. Does nothing when the user agent string does not contain `/kubo/` or `/go-ipfs/`
func CheckVersionOption() ServeOption {
	daemonVersion := version.ApiVersion

	return func(n *core.IpfsNode, l net.Listener, parent *http.ServeMux) (*http.ServeMux, error) {
		mux := http.NewServeMux()
		parent.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, APIPath) {
				cmdqry := r.URL.Path[len(APIPath):]
				pth := strings.Split(cmdqry, "/")

				// backwards compatibility to previous version check
				if len(pth) >= 2 && pth[1] != "version" {
					clientVersion := r.UserAgent()
					// skips check if client is not kubo (go-ipfs)
					if (strings.Contains(clientVersion, "/go-ipfs/") || strings.Contains(clientVersion, "/kubo/")) && daemonVersion != clientVersion {
						http.Error(w, fmt.Sprintf("%s (%s != %s)", errAPIVersionMismatch, daemonVersion, clientVersion), http.StatusBadRequest)
						return
					}
				}
			}

			mux.ServeHTTP(w, r)
		})

		return mux, nil
	}
}
