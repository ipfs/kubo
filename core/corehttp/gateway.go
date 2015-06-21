package corehttp

import (
	"fmt"
	"net/http"
	"sync"
	"os"

	commands "github.com/ipfs/go-ipfs/commands"
	cmdsHttp "github.com/ipfs/go-ipfs/commands/http"
	corecommands "github.com/ipfs/go-ipfs/core/commands"
	core "github.com/ipfs/go-ipfs/core"
	id "github.com/ipfs/go-ipfs/p2p/protocol/identify"
)

// Gateway should be instantiated using NewGateway
type Gateway struct {
	Config GatewayConfig
}

type GatewayConfig struct {
	BlockList *BlockList
	Writable  bool
}

func NewGateway(conf GatewayConfig) *Gateway {
	return &Gateway{
		Config: conf,
	}
}

func (g *Gateway) ServeOption(cctx * commands.Context) ServeOption {
	var commandList = map[*commands.Command]bool{}

	for _, cmd := range corecommands.Root.Subcommands {
		commandList[cmd] = cmd.GatewayAccess
	}

	return func(n *core.IpfsNode, mux *http.ServeMux) (*http.ServeMux, error) {
		gateway, err := newGatewayHandler(n, g.Config)
		if err != nil {
			return nil, err
		}
		mux.Handle("/ipfs/", gateway)
		mux.Handle("/ipns/", gateway)

		if cctx != nil {
			origin := os.Getenv(originEnvKey)
			cmdHandler := cmdsHttp.NewHandler(*cctx, corecommands.Root, origin, commandList)
			mux.Handle(cmdsHttp.ApiPath+"/", cmdHandler)
		}
		return mux, nil
	}
}


func GatewayOption(writable bool, cctx * commands.Context) ServeOption {
	g := NewGateway(GatewayConfig{
		Writable:  writable,
		BlockList: &BlockList{},
	})
	return g.ServeOption(cctx)
}


func VersionOption() ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Client Version:   %s\n", id.ClientVersion)
			fmt.Fprintf(w, "Protocol Version: %s\n", id.IpfsVersion)
		})
		return mux, nil
	}
}

// Decider decides whether to Allow string
type Decider func(string) bool

type BlockList struct {
	mu      sync.RWMutex
	Decider Decider
}

func (b *BlockList) ShouldAllow(s string) bool {
	b.mu.RLock()
	d := b.Decider
	b.mu.RUnlock()
	if d == nil {
		return true
	}
	return d(s)
}

// SetDecider atomically swaps the blocklist's decider. This method is
// thread-safe.
func (b *BlockList) SetDecider(d Decider) {
	b.mu.Lock()
	b.Decider = d
	b.mu.Unlock()
}

func (b *BlockList) ShouldBlock(s string) bool {
	return !b.ShouldAllow(s)
}
