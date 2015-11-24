package gateway

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	core "github.com/ipfs/go-ipfs/core"
	corehttp "github.com/ipfs/go-ipfs/core/corehttp"
	id "github.com/ipfs/go-ipfs/p2p/protocol/identify"
)

// Gateway should be instantiated using NewGateway
type Gateway struct {
	Config GatewayConfig
}

type GatewayConfig struct {
	Headers   map[string][]string
	BlockList *BlockList
	Writable  bool
}

func NewGateway(conf GatewayConfig) *Gateway {
	return &Gateway{
		Config: conf,
	}
}

// extracted from github.com/ipfs/go-ipfs/core/corehttp/corehttp.go
// makeHandler turns a list of ServeOptions into a http.Handler that implements
// all of the given options, in order.
func makeHandler(n *core.IpfsNode, l net.Listener, options ...corehttp.ServeOption) (http.Handler, error) {
	topMux := http.NewServeMux()
	mux := topMux
	for _, option := range options {
		var err error
		mux, err = option(n, l, mux)
		if err != nil {
			return nil, err
		}
	}
	return topMux, nil
}

func (g *Gateway) ServeOption() corehttp.ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		// pass user's HTTP headers
		cfg, err := n.Repo.Config()
		if err != nil {
			return nil, err
		}

		g.Config.Headers = cfg.Gateway.HTTPHeaders

		gateway, err := newGatewayHandler(n, g.Config)
		if err != nil {
			return nil, err
		}
		mux.Handle("/ipfs/", gateway)
		mux.Handle("/ipns/", gateway)
		return mux, nil
	}
}

func GatewayOption(writable bool) corehttp.ServeOption {
	g := NewGateway(GatewayConfig{
		Writable:  writable,
		BlockList: &BlockList{},
	})
	return g.ServeOption()
}

func VersionOption() corehttp.ServeOption {
	return func(n *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
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
