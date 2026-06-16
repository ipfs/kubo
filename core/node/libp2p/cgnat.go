package libp2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/fx"
)

// cgnatNet is RFC 6598 shared address space, used by carrier-grade NAT. The
// range is also used by VPN/overlay tools (Tailscale, ZeroTier), which place it
// on a local interface, so an address here is treated as CGNAT only when it is
// a NAT-mapped WAN address, that is, not one of this node's own interface
// addresses. The same range is filtered by the `server` config profile.
var _, cgnatNet, _ = net.ParseCIDR("100.64.0.0/10")

// cgnatNoticeOut is where the one-time notice is written. A direct write to
// stderr is used instead of the package logger because go-log defaults to
// ERROR level, which would hide a WARN/INFO line, and this is a one-time user
// advisory rather than a recurring log event. Opt out with the
// Internal.CGNATCheck config flag. Overridable in tests.
var cgnatNoticeOut io.Writer = os.Stderr

// natKind classifies a node's NAT situation. Confidence decreases from
// natCGNAT (a NAT-mapped WAN address in the RFC 6598 carrier-NAT range) to
// natDoubleNAT (a NAT-mapped private WAN address, usually but not always an ISP
// CGNAT).
type natKind int

const (
	natUnknown natKind = iota
	natCGNAT
	natDoubleNAT
)

func (k natKind) String() string {
	switch k {
	case natCGNAT:
		return "cgnat"
	case natDoubleNAT:
		return "double-nat"
	default:
		return ""
	}
}

// natInspectHost is the subset of *BasicHost used for NAT classification.
// AllAddrs is not part of the core host.Host interface; it includes the
// NAT-mapped WAN address discovered via UPnP/NAT-PMP/PCP, even when that
// address is in a private or shared range.
type natInspectHost interface {
	AllAddrs() []ma.Multiaddr
	Reachability() network.Reachability
}

// DetectNAT classifies the host's NAT situation from the addresses it knows
// about. It returns "cgnat", "double-nat", or "" (neither, or not
// determinable). Detection is best-effort and conservative: it fires only when
// a private or shared-range address appears as a NAT-mapped WAN address (via
// UPnP/NAT-PMP/PCP) that is not one of this node's own interfaces. When the
// upstream address is hidden (for example a router that does not answer NAT port
// mapping), the node looks like any ordinary NAT and "" is returned.
func DetectNAT(h host.Host) string {
	return detectNATKind(h).String()
}

func detectNATKind(h host.Host) natKind {
	ih, ok := h.(natInspectHost)
	if !ok {
		return natUnknown
	}
	return classifyNAT(ih.AllAddrs(), interfaceIP4Set(h), ih.Reachability())
}

// classifyNAT is the pure core of detection. allAddrs is host.AllAddrs();
// ifaceIPs is the set of IPv4 addresses bound on local interfaces; r
// corroborates. Only addresses that are NOT on a local interface are
// considered: such an address is a NAT-mapped WAN address, which means our
// router's WAN side is itself behind another NAT. On-interface shared-range
// addresses are ignored because VPN/overlay tools (Tailscale, ZeroTier) put
// RFC 6598 addresses on a local interface without any carrier NAT involved.
func classifyNAT(allAddrs []ma.Multiaddr, ifaceIPs map[string]struct{}, r network.Reachability) natKind {
	if r == network.ReachabilityPublic {
		// AutoNAT confirms we are publicly reachable: no NAT problem to report.
		return natUnknown
	}
	kind := natUnknown
	for _, a := range allAddrs {
		ip := addrIP4(a)
		if ip == nil || inIPSet(ifaceIPs, ip) {
			continue // skip non-IPv4 and our own interface addrs (incl. overlays)
		}
		switch {
		case cgnatNet.Contains(ip):
			kind = natCGNAT // RFC 6598 WAN address: carrier-grade NAT
		case ip.IsPrivate() && kind == natUnknown:
			kind = natDoubleNAT // RFC1918 WAN address: double NAT (often carrier)
		}
	}
	return kind
}

// interfaceIP4Set returns the set of IPv4 addresses bound on local
// interfaces, keyed by their string form.
func interfaceIP4Set(h host.Host) map[string]struct{} {
	set := make(map[string]struct{})
	addrs, err := h.Network().InterfaceListenAddresses()
	if err != nil {
		return set
	}
	for _, a := range addrs {
		if ip := addrIP4(a); ip != nil {
			set[ip.String()] = struct{}{}
		}
	}
	return set
}

// addrIP4 returns the IPv4 address of m, or nil if m has no IPv4 component.
func addrIP4(m ma.Multiaddr) net.IP {
	s, err := m.ValueForProtocol(ma.P_IP4)
	if err != nil {
		return nil
	}
	return net.ParseIP(s)
}

func inIPSet(set map[string]struct{}, ip net.IP) bool {
	_, ok := set[ip.String()]
	return ok
}

// MonitorCGNAT logs a one-time notice if the node appears to be behind
// carrier-grade or double NAT. The NAT-mapped WAN address only appears once
// NAT port-mapping discovery completes, so the check re-runs on every
// EvtLocalAddressesUpdated rather than once at startup. The notice is
// diagnostic and never aborts node startup.
//
// Disable with the Internal.CGNATCheck config flag.
func MonitorCGNAT() func(fx.Lifecycle, host.Host) error {
	return func(lc fx.Lifecycle, h host.Host) error {
		sub, err := h.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
		if err != nil {
			log.Errorf("cgnat check: subscribe to EvtLocalAddressesUpdated failed (%s); monitor disabled", err)
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
			// reported is read and written only from this single goroutine, so
			// no synchronization is needed.
			var reported natKind
			check := func() {
				next, emit := latchNotice(reported, detectNATKind(h))
				if !emit {
					return
				}
				reported = next
				if next == natCGNAT {
					logCGNATNotice()
				} else {
					logDoubleNATNotice()
				}
			}
			check() // in case NAT mappings already exist
			for {
				select {
				case <-ctx.Done():
					return
				case _, ok := <-sub.Out():
					if !ok {
						return
					}
					check()
				}
			}
		}()
		return nil
	}
}

// latchNotice decides whether to emit a notice, given the strongest kind
// already reported and the kind just detected. The NAT-mapped WAN address can
// surface only after port-mapping discovery completes, so a weaker double-NAT
// signal may be seen before the stronger, more specific CGNAT one. latchNotice
// allows a single double-nat -> cgnat upgrade but never downgrades or repeats a
// kind already reported.
func latchNotice(reported, detected natKind) (natKind, bool) {
	switch detected {
	case natCGNAT:
		if reported != natCGNAT {
			return natCGNAT, true
		}
	case natDoubleNAT:
		if reported == natUnknown {
			return natDoubleNAT, true
		}
	}
	return reported, false
}

const cgnatNotice = `WARNING: carrier-grade NAT (CGNAT) detected
  Your ISP shares one public address across many subscribers (100.64.0.0/10, RFC 6598).
  - Other peers cannot reach this node directly; it relies on relays and hole punching.
  - A busy node opens many connections (especially UDP/QUIC) that can fill your ISP's shared
    NAT table and disrupt internet for every device on your network.
  If your home internet drops, reduce this node's footprint:
    https://docs.ipfs.tech/how-to/nat-configuration/
  Silence this notice with: ipfs config --json Internal.CGNATCheck false`

const doubleNATNotice = `NOTICE: this node appears to be behind double NAT
  Your router's WAN address is itself private, so another NAT sits above it,
  usually your ISP's (carrier-grade NAT).
  - Other peers cannot reach this node directly; it relies on relays and hole punching.
  - A busy node can fill the upstream shared NAT table and disrupt internet for your network.
  If your home internet drops, reduce this node's footprint:
    https://docs.ipfs.tech/how-to/nat-configuration/
  Silence this notice with: ipfs config --json Internal.CGNATCheck false`

func logCGNATNotice()     { fmt.Fprintln(cgnatNoticeOut, cgnatNotice) }
func logDoubleNATNotice() { fmt.Fprintln(cgnatNoticeOut, doubleNATNotice) }
