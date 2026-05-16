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
