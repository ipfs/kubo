package libp2p

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	logging "github.com/ipfs/go-log/v2"
	version "github.com/ipfs/kubo"
	"github.com/ipfs/kubo/config"
	p2pforge "github.com/ipshipyard/p2p-forge/client"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
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

// Sources for deadListenerFinding.Source.
const (
	deadListenerSourceAddrFilters = "Swarm.AddrFilters"
	deadListenerSourceNoAnnounce  = "Addresses.NoAnnounce"
)

// deadListenerFinding is one resolved listener killed by a CIDR rule:
// `Swarm.AddrFilters` (gater RSTs inbound) or `Addresses.NoAnnounce`
// (listener never advertised).
type deadListenerFinding struct {
	Listener string // resolved listen multiaddr (interface-bound)
	Source   string // deadListenerSourceAddrFilters or deadListenerSourceNoAnnounce
	Rule     string // matching CIDR rule from Source
}

// findDeadListeners returns one finding per (listener, rule, source)
// triple whose IP component falls inside a CIDR in addrFilters or
// noAnnounce.
//
// listenAddrs must be already-resolved interface addresses (output of
// `host.Network().InterfaceListenAddresses()`). Without resolution, the
// unspecified address itself can match a broad filter (`::` is in
// `::/3`) even when the listener accepts globally-routable peers.
//
// NoAnnounce matches on loopback are skipped: stripping loopback from
// identify and DHT records is normal operator intent, not a bug.
// AddrFilters matches on loopback are always reported, since that is
// the misconfiguration this check exists to catch.
//
// Listeners without an IP component (`/dns`, `/dnsaddr`) and
// unparseable rules are skipped silently.
func findDeadListeners(listenAddrs []ma.Multiaddr, addrFilters []string, noAnnounce []string) []deadListenerFinding {
	check := func(source string, rules []string) []deadListenerFinding {
		var out []deadListenerFinding
		for _, r := range rules {
			mask, err := mamask.NewMask(r)
			if err != nil {
				// Malformed CIDR (caught upstream for AddrFilters) or
				// an exact-match multiaddr in NoAnnounce. Skip either way.
				continue
			}
			f := ma.NewFilters()
			f.AddFilter(*mask, ma.ActionDeny)
			for _, l := range listenAddrs {
				if !f.AddrBlocked(l) {
					continue
				}
				if source == deadListenerSourceNoAnnounce && isLoopbackMultiaddr(l) {
					// Suppressing loopback announcement is operator-intent,
					// not a misconfiguration.
					continue
				}
				out = append(out, deadListenerFinding{
					Listener: l.String(),
					Source:   source,
					Rule:     r,
				})
			}
		}
		return out
	}

	findings := check(deadListenerSourceAddrFilters, addrFilters)
	findings = append(findings, check(deadListenerSourceNoAnnounce, noAnnounce)...)
	return findings
}

// isLoopbackMultiaddr reports whether m's IP component is loopback
// (`127.0.0.0/8` or `::1`). Returns false if m has no IP component.
func isLoopbackMultiaddr(m ma.Multiaddr) bool {
	ip, err := manet.ToIP(m)
	if err != nil {
		return false
	}
	return ip.IsLoopback()
}

// logDeadListenerFinding writes one ERROR line per finding, naming
// the listener, the matching CIDR rule, and where to remove it from.
// Each line stands alone so operators can grep and act on it.
func logDeadListenerFinding(f deadListenerFinding) {
	switch f.Source {
	case deadListenerSourceAddrFilters:
		log.Errorf(
			"Addresses.Swarm listener %q matches Swarm.AddrFilters rule %q, "+
				"so Kubo rejects every incoming connection to it. Remove %q "+
				"from Swarm.AddrFilters to allow connections to this listener.",
			f.Listener, f.Rule, f.Rule,
		)
	case deadListenerSourceNoAnnounce:
		log.Errorf(
			"Addresses.Swarm listener %q matches Addresses.NoAnnounce rule %q, "+
				"so Kubo will not advertise it to other peers. Remove %q from "+
				"Addresses.NoAnnounce to advertise this listener.",
			f.Listener, f.Rule, f.Rule,
		)
	}
}

// MonitorDeadListeners runs findDeadListeners at startup and on every
// EvtLocalAddressesUpdated. Listen addresses change at runtime (NAT
// mapping, new interface, AutoTLS cert), so a one-shot check would
// miss listeners that appear later.
//
// Findings are deduplicated against the previous run: a stable
// misconfiguration is logged once.
//
// If subscribing to the event bus fails, the runtime monitor is
// disabled and only the startup check runs. The check is diagnostic
// and must never abort node startup.
func MonitorDeadListeners(addrFilters []string, noAnnounce []string) func(fx.Lifecycle, host.Host) error {
	return func(lc fx.Lifecycle, h host.Host) error {
		seen := make(map[deadListenerFinding]struct{})
		runCheck := func() {
			listenAddrs, err := h.Network().InterfaceListenAddresses()
			if err != nil {
				log.Warnf("dead-listener check: read InterfaceListenAddresses: %s", err)
				return
			}
			next := make(map[deadListenerFinding]struct{})
			for _, f := range findDeadListeners(listenAddrs, addrFilters, noAnnounce) {
				next[f] = struct{}{}
				if _, ok := seen[f]; ok {
					continue
				}
				logDeadListenerFinding(f)
			}
			seen = next
		}

		// Startup check, always runs even if the runtime monitor below
		// cannot be wired up.
		runCheck()

		sub, err := h.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
		if err != nil {
			log.Errorf("dead-listener check: subscribe to EvtLocalAddressesUpdated failed (%s); runtime monitor disabled, startup check already ran", err)
			return nil
		}

		ctx, cancel := context.WithCancel(context.Background())
		lc.Append(fx.Hook{
			OnStop: func(_ context.Context) error {
				cancel()
				return nil
			},
		})

		go func() {
			defer sub.Close()
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-sub.Out():
					if !ok {
						return
					}
					runCheck()
				}
			}
		}()
		return nil
	}
}

func makeAddrsFactory(announce []string, appendAnnounce []string, noAnnounce []string) (p2pbhost.AddrsFactory, error) {
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
	for _, addr := range appendAnnounce {
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
			// Drop empty multiaddrs. Since go-multiaddr v0.15 made
			// Multiaddr a slice type, a zero-value Multiaddr encodes to
			// zero bytes and would otherwise reach the host's signed peer
			// record, where peers render it as "/" and reject the address.
			// See https://github.com/libp2p/js-libp2p/issues/3478#issuecomment-4322093929
			if len(maddr) == 0 {
				continue
			}
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

func AddrsFactory(announce []string, appendAnnounce []string, noAnnounce []string) any {
	return func(params struct {
		fx.In
		ForgeMgr *p2pforge.P2PForgeCertMgr `optional:"true"`
	},
	) (opts Libp2pOpts, err error) {
		var addrsFactory p2pbhost.AddrsFactory
		announceAddrsFactory, err := makeAddrsFactory(announce, appendAnnounce, noAnnounce)
		if err != nil {
			return opts, err
		}
		if params.ForgeMgr == nil {
			addrsFactory = announceAddrsFactory
		} else {
			addrsFactory = func(multiaddrs []ma.Multiaddr) []ma.Multiaddr {
				forgeProcessing := params.ForgeMgr.AddressFactory()(multiaddrs)
				announceProcessing := announceAddrsFactory(forgeProcessing)
				return announceProcessing
			}
		}
		opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
		return
	}
}

func ListenOn(addresses []string) any {
	return func() (opts Libp2pOpts) {
		return Libp2pOpts{
			Opts: []libp2p.Option{
				libp2p.ListenAddrStrings(addresses...),
			},
		}
	}
}

func P2PForgeCertMgr(repoPath string, cfg config.AutoTLS, atlsLog *logging.ZapEventLogger) any {
	return func() (*p2pforge.P2PForgeCertMgr, error) {
		storagePath := filepath.Join(repoPath, "p2p-forge-certs")
		rawLogger := atlsLog.Desugar()

		// TODO: this should not be necessary after
		// https://github.com/ipshipyard/p2p-forge/pull/42 but keep it here for
		// now to help tracking down any remaining conditions causing
		// https://github.com/ipshipyard/p2p-forge/issues/8
		certmagic.Default.Logger = rawLogger.Named("default_fixme")
		certmagic.DefaultACME.Logger = rawLogger.Named("default_acme_client_fixme")

		registrationDelay := cfg.RegistrationDelay.WithDefault(config.DefaultAutoTLSRegistrationDelay)
		if cfg.Enabled == config.True && cfg.RegistrationDelay.IsDefault() {
			// Skip delay if user explicitly enabled AutoTLS.Enabled in config
			// and did not set custom AutoTLS.RegistrationDelay
			registrationDelay = 0 * time.Second
		}

		certStorage := &certmagic.FileStorage{Path: storagePath}
		certMgr, err := p2pforge.NewP2PForgeCertMgr(
			p2pforge.WithLogger(rawLogger.Sugar()),
			p2pforge.WithForgeDomain(cfg.DomainSuffix.WithDefault(config.DefaultDomainSuffix)),
			p2pforge.WithForgeRegistrationEndpoint(cfg.RegistrationEndpoint.WithDefault(config.DefaultRegistrationEndpoint)),
			p2pforge.WithRegistrationDelay(registrationDelay),
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
