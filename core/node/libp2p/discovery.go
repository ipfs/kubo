package libp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	legacymdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns_legacy"

	"go.uber.org/fx"

	"github.com/ipfs/go-ipfs/core/node/helpers"
)

const discoveryConnTimeout = time.Second * 30

type discoveryHandler struct {
	ctx  context.Context
	host host.Host
}

func (dh *discoveryHandler) HandlePeerFound(p peer.AddrInfo) {
	log.Info("connecting to discovered peer: ", p)
	ctx, cancel := context.WithTimeout(dh.ctx, discoveryConnTimeout)
	defer cancel()
	if err := dh.host.Connect(ctx, p); err != nil {
		log.Warnf("failed to connect to peer %s found by discovery: %s", p.ID, err)
	}
}

func DiscoveryHandler(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host) *discoveryHandler {
	return &discoveryHandler{
		ctx:  helpers.LifecycleCtx(mctx, lc),
		host: host,
	}
}

func SetupDiscovery(useMdns bool, mdnsInterval int) func(helpers.MetricsCtx, fx.Lifecycle, host.Host, *discoveryHandler) error {
	return func(mctx helpers.MetricsCtx, lc fx.Lifecycle, host host.Host, handler *discoveryHandler) error {
		if useMdns {
			service := mdns.NewMdnsService(host, mdns.ServiceName, handler)
			if err := service.Start(); err != nil {
				return err
			}

			if mdnsInterval == 0 {
				mdnsInterval = 5
			}
			legacyService, err := legacymdns.NewMdnsService(mctx, host, time.Duration(mdnsInterval)*time.Second, legacymdns.ServiceTag)
			if err != nil {
				log.Error("mdns error: ", err)
				return nil
			}
			legacyService.RegisterNotifee(handler)
		}
		return nil
	}
}
