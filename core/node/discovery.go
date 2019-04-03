package node

import (
	"context"
	"time"

	"github.com/ipfs/go-ipfs-config"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"go.uber.org/fx"
)

const discoveryConnTimeout = time.Second * 30

type discoveryHandler struct {
	ctx  context.Context
	host host.Host
}

func (dh *discoveryHandler) HandlePeerFound(p peerstore.PeerInfo) {
	log.Warning("trying peer info: ", p)
	ctx, cancel := context.WithTimeout(dh.ctx, discoveryConnTimeout)
	defer cancel()
	if err := dh.host.Connect(ctx, p); err != nil {
		log.Warning("Failed to connect to peer found by discovery: ", err)
	}
}

func NewDiscoveryHandler(mctx MetricsCtx, lc fx.Lifecycle, host host.Host) *discoveryHandler {
	return &discoveryHandler{
		ctx:  lifecycleCtx(mctx, lc),
		host: host,
	}
}

func SetupDiscovery(mctx MetricsCtx, lc fx.Lifecycle, cfg *config.Config, host host.Host, handler *discoveryHandler) error {
	if cfg.Discovery.MDNS.Enabled {
		mdns := cfg.Discovery.MDNS
		if mdns.Interval == 0 {
			mdns.Interval = 5
		}
		service, err := discovery.NewMdnsService(lifecycleCtx(mctx, lc), host, time.Duration(mdns.Interval)*time.Second, discovery.ServiceTag)
		if err != nil {
			log.Error("mdns error: ", err)
			return nil
		}
		service.RegisterNotifee(handler)
	}
	return nil
}
