package corehttp

import (
	"net/http"
	"sync"

	core "github.com/jbenet/go-ipfs/core"
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

func (g *Gateway) ServeOption() ServeOption {
	return func(n *core.IpfsNode, mux *http.ServeMux) error {
		gateway, err := newGatewayHandler(n, g.Config)
		if err != nil {
			return err
		}
		mux.Handle("/ipfs/", gateway)
		mux.Handle("/ipns/", gateway)
		return nil
	}
}

func GatewayOption(writable bool) ServeOption {
	g := NewGateway(GatewayConfig{
		Writable:  writable,
		BlockList: &BlockList{},
	})
	return g.ServeOption()
}

// Decider decides whether to Allow string
type Decider func(string) bool

type BlockList struct {
	mu sync.RWMutex
	d  Decider
}

func (b *BlockList) ShouldAllow(s string) bool {
	b.mu.RLock()
	d := b.d
	b.mu.RUnlock()
	if d == nil {
		return true
	}
	return d(s)
}

// SetDecider atomically swaps the blocklist's decider
func (b *BlockList) SetDecider(d Decider) {
	b.mu.Lock()
	b.d = d
	b.mu.Unlock()
}

func (b *BlockList) ShouldBlock(s string) bool {
	return !b.ShouldAllow(s)
}
