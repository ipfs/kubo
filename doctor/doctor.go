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

	netroute "github.com/libp2p/go-netroute"

	config "github.com/ipfs/go-ipfs-config"
	logging "github.com/ipfs/go-log"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var log = logging.Logger("namesys")

type Doctor struct {
	host    libp2p.Host
	config  *config.Config
	console io.Writer

	mu                     sync.RWMutex
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
	})
	if err != nil {
		return err
	}

	evtCh := n.sub.Out()
	go func() {
		defer n.Close()

		for evt := range evtCh {
			switch evt := evt.(type) {
			case libp2pEvent.EvtLocalReachabilityChanged:
				n.handleReachability(evt)
			case libp2pEvent.EvtLocalAddressesUpdated:
				n.handleAddrUpdate(evt)
			default:
				log.Errorf("unexpected event type: %T", evt)
			}
		}
	}()
	return nil
}

func (n *Doctor) printf(f string, rest ...interface{}) {
	fmt.Fprintf(n.console, f+"\n", rest...)
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
	if !n.hasNagged {
		n.hasNagged = true
		n.printf("NOTE: Your node appears to be behind a firewall. Please run `ipfs diag net` to diagnose.")
	}
}

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

func (n *Doctor) GetStatus(ctx context.Context) (*Status, error) {
	n.mu.RLock()

	status := &Status{
		Reachability:     n.reachability,
		AutoRelayEnabled: n.config.Swarm.EnableAutoRelay && !n.config.Swarm.EnableRelayHop,
		UsingRelays:      n.usingRelays,
	}

	n.mu.RUnlock()

	status.Listening = len(n.host.Network().ListenAddresses()) > 0

	addrs, err := n.host.Network().InterfaceListenAddresses()
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
	for _, addr := range addrs {
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

	return status, nil
}
