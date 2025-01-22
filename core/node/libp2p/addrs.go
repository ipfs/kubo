package libp2p

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	logging "github.com/ipfs/go-log"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	p2pforge "github.com/ipshipyard/p2p-forge/client"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	mamask "github.com/whyrusleeping/multiaddr-filter"

	"github.com/caddyserver/certmagic"
	"go.uber.org/fx"
)

func AddrFilters(filters []string) func() (*ma.Filters, Libp2pOpts, error) {
	return func() (filter *ma.Filters, opts Libp2pOpts, err error) {
		filter = ma.NewFilters()
		opts.Opts = append(opts.Opts, libp2p.ConnectionGater((*filtersConnectionGater)(filter)))
		for _, s := range filters {
			f, err := mamask.NewMask(s)
			if err != nil {
				return filter, opts, fmt.Errorf("incorrectly formatted address filter in config: %s", s)
			}
			filter.AddFilter(*f, ma.ActionDeny)
		}
		return filter, opts, nil
	}
}

func makeAddrsFactory(announce []string, appendAnnouce []string, noAnnounce []string) (p2pbhost.AddrsFactory, error) {
	var err error                     // To assign to the slice in the for loop
	existing := make(map[string]bool) // To avoid duplicates

	annAddrs := make([]ma.Multiaddr, len(announce))
	for i, addr := range announce {
		annAddrs[i], err = ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		existing[addr] = true
	}

	var appendAnnAddrs []ma.Multiaddr
	for _, addr := range appendAnnouce {
		if existing[addr] {
			// skip AppendAnnounce that is on the Announce list already
			continue
		}
		appendAddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		appendAnnAddrs = append(appendAnnAddrs, appendAddr)
	}

	filters := ma.NewFilters()
	noAnnAddrs := map[string]bool{}
	for _, addr := range noAnnounce {
		f, err := mamask.NewMask(addr)
		if err == nil {
			filters.AddFilter(*f, ma.ActionDeny)
			continue
		}
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		noAnnAddrs[string(maddr.Bytes())] = true
	}

	return func(allAddrs []ma.Multiaddr) []ma.Multiaddr {
		var addrs []ma.Multiaddr
		if len(annAddrs) > 0 {
			addrs = annAddrs
		} else {
			addrs = allAddrs
		}
		addrs = append(addrs, appendAnnAddrs...)

		var out []ma.Multiaddr
		for _, maddr := range addrs {
			// check for exact matches
			ok := noAnnAddrs[string(maddr.Bytes())]
			// check for /ipcidr matches
			if !ok && !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}, nil
}

func AddrsFactory(announce []string, appendAnnouce []string, noAnnounce []string) interface{} {
	return func(params struct {
		fx.In
		ForgeMgr *p2pforge.P2PForgeCertMgr `optional:"true"`
	},
	) (opts Libp2pOpts, err error) {
		var addrsFactory p2pbhost.AddrsFactory
		announceAddrsFactory, err := makeAddrsFactory(announce, appendAnnouce, noAnnounce)
		if err != nil {
			return opts, err
		}
		if params.ForgeMgr == nil {
			addrsFactory = announceAddrsFactory
		} else {
			addrsFactory = func(multiaddrs []ma.Multiaddr) []ma.Multiaddr {
				forgeProcessing := params.ForgeMgr.AddressFactory()(multiaddrs)
				annouceProcessing := announceAddrsFactory(forgeProcessing)
				return annouceProcessing
			}
		}
		opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
		return
	}
}

func ListenOn(addresses []string) interface{} {
	return func() (opts Libp2pOpts) {
		return Libp2pOpts{
			Opts: []libp2p.Option{
				libp2p.ListenAddrStrings(addresses...),
			},
		}
	}
}

func P2PForgeCertMgr(repoPath string, cfg config.AutoTLS, atlsLog *logging.ZapEventLogger) interface{} {
	return func() (*p2pforge.P2PForgeCertMgr, error) {
		storagePath := filepath.Join(repoPath, "p2p-forge-certs")
		rawLogger := atlsLog.Desugar()

		// TODO: this should not be necessary after
		// https://github.com/ipshipyard/p2p-forge/pull/42 but keep it here for
		// now to help tracking down any remaining conditions causing
		// https://github.com/ipshipyard/p2p-forge/issues/8
		certmagic.Default.Logger = rawLogger.Named("default_fixme")
		certmagic.DefaultACME.Logger = rawLogger.Named("default_acme_client_fixme")

		certStorage := &certmagic.FileStorage{Path: storagePath}
		certMgr, err := p2pforge.NewP2PForgeCertMgr(
			p2pforge.WithLogger(rawLogger.Sugar()),
			p2pforge.WithForgeDomain(cfg.DomainSuffix.WithDefault(config.DefaultDomainSuffix)),
			p2pforge.WithForgeRegistrationEndpoint(cfg.RegistrationEndpoint.WithDefault(config.DefaultRegistrationEndpoint)),
			p2pforge.WithCAEndpoint(cfg.CAEndpoint.WithDefault(config.DefaultCAEndpoint)),
			p2pforge.WithForgeAuth(cfg.RegistrationToken.WithDefault(os.Getenv(p2pforge.ForgeAuthEnv))),
			p2pforge.WithUserAgent(version.GetUserAgentVersion()),
			p2pforge.WithCertificateStorage(certStorage),
			p2pforge.WithShortForgeAddrs(cfg.ShortAddrs.WithDefault(config.DefaultAutoTLSShortAddrs)),
		)
		if err != nil {
			return nil, err
		}

		return certMgr, nil
	}
}

func StartP2PAutoTLS(lc fx.Lifecycle, certMgr *p2pforge.P2PForgeCertMgr, h host.Host) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			certMgr.ProvideHost(h)
			return certMgr.Start()
		},
		OnStop: func(ctx context.Context) error {
			certMgr.Stop()
			return nil
		},
	})
}
