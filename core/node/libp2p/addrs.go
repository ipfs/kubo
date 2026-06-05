package libp2p

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

func AddrsFactory(announce []string, appendAnnounce []string, noAnnounce []string, announceHTTPProvider bool) any {
	return func(params struct {
		fx.In
		ForgeMgr *p2pforge.P2PForgeCertMgr `optional:"true"`
	},
	) (opts Libp2pOpts, err error) {
		announceAddrsFactory, err := makeAddrsFactory(announce, appendAnnounce, noAnnounce)
		if err != nil {
			return opts, err
		}
		// The factory pipeline runs in this order so each step sees the
		// output of the previous one:
		//   1. ForgeMgr substitutes the wildcard SNI in /tls/sni/*.<domain>/ws
		//      with the real per-peer hostname (or its short form).
		//   2. HTTPProvider derives an HTTP-flavored multiaddr from each /ws
		//      so HTTP retrieval clients can discover this peer; runs after
		//      ForgeMgr so the SNI value in /tls/sni/<host>/http is already
		//      the resolved one, not the wildcard.
		//   3. announceAddrsFactory applies Addresses.Announce/AppendAnnounce
		//      and drops anything matching Addresses.NoAnnounce, so the
		//      derived addresses are filtered just like every other one.
		addrsFactory := func(multiaddrs []ma.Multiaddr) []ma.Multiaddr {
			if params.ForgeMgr != nil {
				multiaddrs = params.ForgeMgr.AddressFactory()(multiaddrs)
			}
			if announceHTTPProvider {
				multiaddrs = appendHTTPProviderAddrs(multiaddrs)
			}
			return announceAddrsFactory(multiaddrs)
		}
		opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
		return
	}
}

// httpComponent is the /http multiaddr protocol component that pairs with
// /ws on the same TCP port. We never listen on it; it is only ever appended
// to announced multiaddrs to advertise the HTTPProvider endpoint.
var httpComponent, _ = ma.NewComponent("http", "")

// appendHTTPProviderAddrs returns a slice that contains every input
// multiaddr plus, for each one ending in /ws, an additional copy with /ws
// replaced by /http. Order is preserved; the /http variant immediately
// follows its /ws sibling. Multiaddrs that do not end in /ws pass through
// unchanged. Duplicates (which would arise if both /ws and /http were
// already in the input) are dropped.
func appendHTTPProviderAddrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	out := make([]ma.Multiaddr, 0, len(addrs)*2)
	seen := make(map[string]struct{}, len(addrs)*2)
	for _, a := range addrs {
		if _, ok := seen[string(a.Bytes())]; !ok {
			out = append(out, a)
			seen[string(a.Bytes())] = struct{}{}
		}
		http, ok := wsToHTTP(a)
		if !ok {
			continue
		}
		if _, ok := seen[string(http.Bytes())]; !ok {
			out = append(out, http)
			seen[string(http.Bytes())] = struct{}{}
		}
	}
	return out
}

// wsToHTTP returns m with its trailing /ws component replaced by /http,
// and a boolean indicating whether the input ended in /ws. Everything
// before the trailing /ws (including any /tls and /tls/sni/<host>
// components) is preserved unchanged.
func wsToHTTP(m ma.Multiaddr) (ma.Multiaddr, bool) {
	prefix, last := ma.SplitLast(m)
	if last == nil || last.Protocol().Code != ma.P_WS {
		return nil, false
	}
	return prefix.AppendComponent(httpComponent), true
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

		// Pull the optional ?dial= and ?dns= test/debug overrides off the
		// URL before handing it to p2p-forge. See docs/config.md.
		regEndpoint, overrides, err := parseForgeOverrides(cfg.RegistrationEndpoint.WithDefault(config.DefaultRegistrationEndpoint))
		if err != nil {
			return nil, fmt.Errorf("AutoTLS.RegistrationEndpoint: %w", err)
		}

		certStorage := &certmagic.FileStorage{Path: storagePath}
		opts := []p2pforge.P2PForgeCertMgrOptions{
			p2pforge.WithLogger(rawLogger.Sugar()),
			p2pforge.WithForgeDomain(cfg.DomainSuffix.WithDefault(config.DefaultDomainSuffix)),
			p2pforge.WithForgeRegistrationEndpoint(regEndpoint),
			p2pforge.WithRegistrationDelay(registrationDelay),
			p2pforge.WithCAEndpoint(cfg.CAEndpoint.WithDefault(config.DefaultCAEndpoint)),
			p2pforge.WithForgeAuth(cfg.RegistrationToken.WithDefault(os.Getenv(p2pforge.ForgeAuthEnv))),
			p2pforge.WithUserAgent(version.GetUserAgentVersion()),
			p2pforge.WithCertificateStorage(certStorage),
			p2pforge.WithShortForgeAddrs(cfg.ShortAddrs.WithDefault(config.DefaultAutoTLSShortAddrs)),
		}
		// AutoTLS.TrustedCARootsPEM: optional CA bundle for private or
		// self-hosted ACME deployments (including the in-process Pebble
		// used by the AutoTLS E2E test).
		if pem := cfg.TrustedCARootsPEM.WithDefault(""); pem != "" {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return nil, fmt.Errorf("AutoTLS.TrustedCARootsPEM did not contain any parseable certificate")
			}
			opts = append(opts, p2pforge.WithTrustedRoots(pool))
		}
		// AutoTLS.AllowPrivateForgeAddrs: lift the "must be publicly
		// reachable" gate on cert requests, for private/intranet
		// deployments and the AutoTLS E2E test (loopback only).
		if cfg.AllowPrivateForgeAddrs.WithDefault(false) {
			opts = append(opts, p2pforge.WithAllowPrivateForgeAddrs())
		}
		if overrides.dial != "" {
			opts = append(opts, p2pforge.WithHTTPClient(forgeDialOverrideClient(overrides.dial)))
		}
		if overrides.dns != "" {
			opts = append(opts, p2pforge.WithResolver(forgeDNSOverrideResolver(overrides.dns)))
		}
		certMgr, err := p2pforge.NewP2PForgeCertMgr(opts...)
		if err != nil {
			return nil, err
		}

		return certMgr, nil
	}
}

// forgeOverrides holds the test/debug overrides parsed off
// AutoTLS.RegistrationEndpoint. All fields are empty when the URL carries no
// overrides. See docs/config.md for the user-facing description.
type forgeOverrides struct {
	dial string // ?dial=host:port: forces the registration POST to dial here
	dns  string // ?dns=host:port: forces DNS-01 pre-flight lookups to this server
}

// parseForgeOverrides strips the recognized ?dial= and ?dns= query parameters
// from raw, returning the cleaned URL and the parsed overrides. Unrelated
// query parameters are preserved.
func parseForgeOverrides(raw string) (cleanURL string, ov forgeOverrides, err error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", forgeOverrides{}, err
	}
	q := u.Query()
	for _, p := range []struct {
		key string
		dst *string
	}{
		{"dial", &ov.dial},
		{"dns", &ov.dns},
	} {
		v := q.Get(p.key)
		if v == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(v); err != nil {
			return "", forgeOverrides{}, fmt.Errorf("%s=%q must be host:port: %w", p.key, v, err)
		}
		*p.dst = v
		q.Del(p.key)
	}
	// Return raw unchanged when no override was set, so a no-op call
	// doesn't reorder unrelated query parameters via Query().Encode().
	if ov == (forgeOverrides{}) {
		return raw, ov, nil
	}
	u.RawQuery = q.Encode()
	return u.String(), ov, nil
}

// forgeDialOverrideClient returns an *http.Client whose Transport always dials
// dialAddr, ignoring the request URL's host. Used by the ?dial= override.
func forgeDialOverrideClient(dialAddr string) *http.Client {
	dialer := &net.Dialer{}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp", dialAddr)
			},
		},
	}
}

// forgeDNSOverrideResolver returns a *net.Resolver that sends every lookup to
// dnsAddr instead of the system resolver. Used by the ?dns= override.
func forgeDNSOverrideResolver(dnsAddr string) *net.Resolver {
	dialer := &net.Dialer{}
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, dnsAddr)
		},
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
