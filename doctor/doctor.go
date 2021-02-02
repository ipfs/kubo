package doctor

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	libp2p "github.com/libp2p/go-libp2p-core"
	libp2pEvent "github.com/libp2p/go-libp2p-core/event"
	libp2pNetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-netroute"
	manet "github.com/multiformats/go-multiaddr/net"

	config "github.com/ipfs/go-ipfs-config"
	logging "github.com/ipfs/go-log"

	ma "github.com/multiformats/go-multiaddr"
)

var log = logging.Logger("/ipfs/diag/doctor")

type Doctor struct {
	host    libp2p.Host
	config  *config.Config
	console io.Writer

	mu sync.RWMutex

	tcpNATDeviceType libp2pNetwork.NATDeviceType
	udpNATDeviceType libp2pNetwork.NATDeviceType

	sub                    libp2pEvent.Subscription
	reachability           libp2pNetwork.Reachability
	hasNagged, usingRelays bool
}

type Status struct {
	Reachability       libp2pNetwork.Reachability
	AutoRelayEnabled   bool
	UsingRelays        bool
	Listening          bool
	TCPPorts, UDPPorts []int
	LocalIP            net.IP
	Gateway            *url.URL
	TCPNATDeviceType   *libp2pNetwork.NATDeviceType
	UDPNATDeviceType   *libp2pNetwork.NATDeviceType
}

func NewDoctor(host libp2p.Host, cfg *config.Config, console io.Writer) *Doctor {
	return &Doctor{
		host:    host,
		config:  cfg,
		console: console,
	}
}

func (n *Doctor) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.sub == nil {
		return nil
	}
	_ = n.sub.Close()
	n.sub = nil
	return nil
}

func (n *Doctor) GetNATStatus() libp2pNetwork.Reachability {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.reachability
}

func (n *Doctor) GetRelayStatus() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.usingRelays
}

func (n *Doctor) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.sub != nil {
		return fmt.Errorf("already started")
	}

	var err error
	n.sub, err = n.host.EventBus().Subscribe([]interface{}{
		new(libp2pEvent.EvtLocalReachabilityChanged),
		new(libp2pEvent.EvtLocalAddressesUpdated),
		new(libp2pEvent.EvtNATDeviceTypeChanged),
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to libp2p events: %s", err)
	}

	evtCh := n.sub.Out()
	go func() {
		// close the doctor and the subscription if we haven't already done so
		defer n.Close()

		for evt := range evtCh {
			switch evt := evt.(type) {
			case libp2pEvent.EvtLocalReachabilityChanged:
				n.handleReachability(evt)
			case libp2pEvent.EvtLocalAddressesUpdated:
				n.handleAddrUpdate(evt)
			case libp2pEvent.EvtNATDeviceTypeChanged:
				n.handleNATDeviceTypeChanged(evt)
			default:
				log.Errorf("unexpected event type: %T", evt)
			}
		}
	}()

	return nil
}

func (n *Doctor) GetStatus(ctx context.Context) (*Status, error) {
	n.mu.RLock()

	status := &Status{
		Reachability:     n.reachability,
		AutoRelayEnabled: n.config.Swarm.EnableAutoRelay && !n.config.Swarm.EnableRelayHop,
		UsingRelays:      n.usingRelays,
	}

	n.mu.RUnlock()

	status.Listening = len(n.host.Network().ListenAddresses()) > 0

	iAddrs, err := n.host.Network().InterfaceListenAddresses()
	if err != nil {
		return status, err
	}

	router, err := netroute.New()
	if err != nil {
		return status, err
	}
	_, gw, src, err := router.Route(net.IPv4zero)
	if err != nil {
		return status, err
	}

	status.LocalIP = src
	for _, addr := range iAddrs {
		addr, err := manet.ToNetAddr(addr)
		if err != nil {
			continue
		}
		switch addr := addr.(type) {
		case *net.TCPAddr:
			if addr.IP.Equal(src) {
				status.TCPPorts = append(status.TCPPorts, addr.Port)
			}
		case *net.UDPAddr:
			if addr.IP.Equal(src) {
				status.UDPPorts = append(status.UDPPorts, addr.Port)
			}
		}
	}

	status.Gateway, _ = testHttp(ctx, gw)

	if n.tcpNATDeviceType != libp2pNetwork.NATDeviceTypeUnknown {
		status.TCPNATDeviceType = &n.tcpNATDeviceType
	}

	if n.udpNATDeviceType != libp2pNetwork.NATDeviceTypeUnknown {
		status.UDPNATDeviceType = &n.udpNATDeviceType
	}

	return status, nil
}

func (n *Doctor) handleNATDeviceTypeChanged(evt libp2pEvent.EvtNATDeviceTypeChanged) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if evt.NatDeviceType == libp2pNetwork.NATDeviceTypeCone {
		fmt.Printf("\n Your NAT device supports NAT traversal via hole punching for %s connections\n", evt.TransportProtocol)
	} else {
		fmt.Printf("\n Your NAT device does NOT support NAT traversal via hole punching for %s connections\n", evt.TransportProtocol)
	}

	switch evt.TransportProtocol {
	case libp2pNetwork.NATTransportUDP:
		n.udpNATDeviceType = evt.NatDeviceType
	case libp2pNetwork.NATTransportTCP:
		n.tcpNATDeviceType = evt.NatDeviceType
	}
}

func (n *Doctor) handleAddrUpdate(evt libp2pEvent.EvtLocalAddressesUpdated) {
	usingRelays := false
	for _, addrUpdate := range evt.Current {
		if _, err := addrUpdate.Address.ValueForProtocol(ma.P_CIRCUIT); err == nil {
			usingRelays = true
			break
		}
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if usingRelays != n.usingRelays {
		n.usingRelays = usingRelays
		status := "not using relays"
		if usingRelays {
			status = "using relays"
		}
		n.printf("Relay Status: %s", status)
	}
}

func (n *Doctor) handleReachability(evt libp2pEvent.EvtLocalReachabilityChanged) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if evt.Reachability == n.reachability {
		// Otherwise, we'll print
		// "unknown" on start.
		return
	}

	n.reachability = evt.Reachability
	status := "unknown"
	switch n.reachability {
	case libp2pNetwork.ReachabilityPrivate:
		status = "private"
	case libp2pNetwork.ReachabilityPublic:
		status = "public"
	default:
	}

	n.printf("Firewall Status: %s", status)

	// We do this simply so users actually run the command.
	if !n.hasNagged {
		n.hasNagged = true
		n.printf("NOTE: Your node appears to be behind a firewall. Please run `ipfs diag net` to diagnose.")
	}
}

func (n *Doctor) printf(f string, rest ...interface{}) {
	fmt.Fprintf(n.console, f+"\n", rest...)
}

// TODO Test this
func testHttp(ctx context.Context, ip net.IP) (*url.URL, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Try the gateway.
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"http://"+(&net.TCPAddr{IP: ip, Port: 80}).String(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http error: %s", resp.Status)
	}
	url, err := resp.Location()
	if err == http.ErrNoLocation {
		return resp.Request.URL, nil
	}
	return url, err
}
