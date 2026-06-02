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

// deadListenerFinding is one resolved listener whose IP falls inside a
// CIDR in `Swarm.AddrFilters` (gater RSTs inbound) or
// `Addresses.NoAnnounce` (listener never advertised).
type deadListenerFinding struct {
	Listener string // resolved listen multiaddr (interface-bound)
	Rule     string // matching CIDR rule from Source
	Source   string // deadListenerSourceAddrFilters or deadListenerSourceNoAnnounce
	Explicit bool   // true when the listener IP+port was bound by a specific-IP entry in `Addresses.Swarm`, not a wildcard expansion
}

// findDeadListeners returns one finding per (listener, rule, source)
// triple whose IP component falls inside a CIDR in addrFilters or
// noAnnounce.
//
// listenAddrs must be already-resolved interface addresses (output of
// `host.Network().InterfaceListenAddresses()`).
//
// swarmListen is the raw `Addresses.Swarm` config. A finding is marked
// `Explicit` when the resolved listener shares its IP and port with a
// specific-IP entry in `swarmListen`, and non-explicit when it came from a
// wildcard listen (`/ip4/0.0.0.0`, `/ip6/::`) expanding onto a per-interface
// address. See explicitListens for why the match is on IP+port rather than
// the full multiaddr string.
//
// Callers route findings to log levels based on Source + Explicit:
//
//   - AddrFilters + Explicit: ERROR. The whole listener is unreachable.
//   - AddrFilters + wildcard: DEBUG. Other interfaces still serve.
//   - NoAnnounce: DEBUG. Operator intent, but useful when tracing why
//     an interface address never reaches identify / DHT records.
//
// Unparseable rules (including exact-match multiaddrs in NoAnnounce)
// and listeners without an IP component are skipped silently.
func findDeadListeners(listenAddrs []ma.Multiaddr, swarmListen, addrFilters, noAnnounce []string) []deadListenerFinding {
	explicit := explicitListens(swarmListen)
	check := func(source string, rules []string) []deadListenerFinding {
		var out []deadListenerFinding
		for _, r := range rules {
			mask, err := mamask.NewMask(r)
			if err != nil {
				continue
			}
			f := ma.NewFilters()
			f.AddFilter(*mask, ma.ActionDeny)
			for _, l := range listenAddrs {
				if !f.AddrBlocked(l) {
					continue
				}
				isExplicit := false
				if ep, ok := listenEndpoint(l); ok {
					_, isExplicit = explicit[ep]
				}
				out = append(out, deadListenerFinding{
					Listener: l.String(),
					Rule:     r,
					Source:   source,
					Explicit: isExplicit,
				})
			}
		}
		return out
	}
	findings := check(deadListenerSourceAddrFilters, addrFilters)
	findings = append(findings, check(deadListenerSourceNoAnnounce, noAnnounce)...)
	return findings
}

// listenEndpoint returns a key identifying m's bound socket: its IP value,
// transport (tcp or udp), and port. The bool is false when m has no specific
// IP (a wildcard such as `/ip4/0.0.0.0`, or an IP-less `/dns...` listen) or
// no TCP/UDP port, since such an address cannot name a single socket.
//
// The key intentionally drops everything after the port. The same socket is
// reported under different multiaddrs depending on transport: the WebSocket
// listener canonicalizes `/wss` to `/tls/ws`, and WebTransport appends a
// `/certhash/...` component for its self-signed certificate. Comparing on
// IP+transport+port keeps a specific-IP listen recognizable across those
// rewrites, where a full-string comparison would not.
//
// The transport is part of the key because TCP and QUIC routinely share a
// port number (Kubo defaults to 4001 for both) yet are distinct sockets. The
// IP is matched by value: an `/ip6zone` qualifier is dropped, so a zoneless
// config entry still matches the resolved interface address.
func listenEndpoint(m ma.Multiaddr) (string, bool) {
	ip, err := manet.ToIP(m)
	if err != nil || ip.IsUnspecified() {
		return "", false
	}
	if port, err := m.ValueForProtocol(ma.P_TCP); err == nil {
		return ip.String() + "/tcp/" + port, true
	}
	if port, err := m.ValueForProtocol(ma.P_UDP); err == nil {
		return ip.String() + "/udp/" + port, true
	}
	return "", false
}

// explicitListens returns the set of network endpoints (IP+port), keyed by
// listenEndpoint, that `Addresses.Swarm` binds to a specific interface.
// Wildcard listens (`/ip4/0.0.0.0`, `/ip6/::`) and entries without an IP
// component (`/dns...`) are skipped: they do not pin a single interface.
//
// A resolved listener counts as explicit when its endpoint is in this set.
// A wildcard listen expands to per-interface addresses whose IPs never
// appear here, so endpoint membership separates a deliberately-bound
// listener from an incidental wildcard expansion onto a filtered interface,
// even when the two share an IP (their transport or port differs).
//
// A `/tcp/0` (OS-assigned port) listen is stored with port "0", which no
// resolved listener reports, so it falls back to non-explicit (DEBUG). The
// reverse-proxy misconfiguration this routing exists to flag always pins a
// fixed port, so the best-effort gap costs nothing in practice.
func explicitListens(swarmListen []string) map[string]struct{} {
	set := make(map[string]struct{}, len(swarmListen))
	for _, s := range swarmListen {
		m, err := ma.NewMultiaddr(s)
		if err != nil {
			continue
		}
		if ep, ok := listenEndpoint(m); ok {
			set[ep] = struct{}{}
		}
	}
	return set
}

// logDeadListenerFinding writes one log line per finding, naming the
// listener, the matching CIDR rule, and where to remove it from. The
// log level depends on the finding's Source and whether the operator
// explicitly bound the listener IP. See findDeadListeners.
func logDeadListenerFinding(f deadListenerFinding) {
	switch {
	case f.Source == deadListenerSourceAddrFilters && f.Explicit:
		log.Errorf(
			"Addresses.Swarm listener %q matches Swarm.AddrFilters rule %q, "+
				"so Kubo rejects every incoming connection to it. Remove %q "+
				"from Swarm.AddrFilters to allow connections to this listener.",
			f.Listener, f.Rule, f.Rule,
		)
	case f.Source == deadListenerSourceAddrFilters:
		log.Debugf(
			"Swarm.AddrFilters rule %q blocks resolved listener %q (from a "+
				"wildcard listen). Other interfaces unaffected.",
			f.Rule, f.Listener,
		)
	case f.Source == deadListenerSourceNoAnnounce:
		log.Debugf(
			"Addresses.NoAnnounce rule %q strips listener %q from "+
				"announcements (identify, DHT self-record).",
			f.Rule, f.Listener,
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
func MonitorDeadListeners(swarmListen, addrFilters, noAnnounce []string) func(fx.Lifecycle, host.Host) error {
	return func(lc fx.Lifecycle, h host.Host) error {
		seen := make(map[deadListenerFinding]struct{})
		runCheck := func() {
			listenAddrs, err := h.Network().InterfaceListenAddresses()
			if err != nil {
				log.Warnf("dead-listener check: read InterfaceListenAddresses: %s", err)
				return
			}
			next := make(map[deadListenerFinding]struct{})
			for _, f := range findDeadListeners(listenAddrs, swarmListen, addrFilters, noAnnounce) {
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
