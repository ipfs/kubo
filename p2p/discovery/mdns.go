package discovery

import (
	"errors"
	"io/ioutil"
	golog "log"
	"net"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/cryptix/mdns"
	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	"github.com/ipfs/go-ipfs/p2p/host"
	"github.com/ipfs/go-ipfs/p2p/peer"
)

const ServiceTag = "discovery.ipfs.io"

type mdnsService struct {
	server  *mdns.Server
	service *mdns.MDNSService
	host    host.Host

	lk       sync.Mutex
	notifees []Notifee
	interval time.Duration
}

func getDialableListenAddrs(ph host.Host) ([]*net.TCPAddr, error) {
	var out []*net.TCPAddr
	for _, addr := range ph.Addrs() {
		na, err := manet.ToNetAddr(addr)
		if err != nil {
			continue
		}
		tcp, ok := na.(*net.TCPAddr)
		if ok {
			out = append(out, tcp)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("failed to find good external addr from peerhost")
	}
	return out, nil
}

func NewMdnsService(peerhost host.Host, interval time.Duration) (Service, error) {

	// TODO: dont let mdns use logging...
	golog.SetOutput(ioutil.Discard)

	var ipaddrs []net.IP
	port := 4001

	addrs, err := getDialableListenAddrs(peerhost)
	if err != nil {
		log.Warning(err)
	} else {
		port = addrs[0].Port
		for _, a := range addrs {
			ipaddrs = append(ipaddrs, a.IP)
		}
	}

	myid := peerhost.ID().Pretty()

	info := []string{myid}
	service, err := mdns.NewMDNSService(myid, ServiceTag, "", "", port, ipaddrs, info)
	if err != nil {
		return nil, err
	}

	// Create the mDNS server, defer shutdown
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, err
	}

	s := &mdnsService{
		server:   server,
		service:  service,
		host:     peerhost,
		interval: interval,
	}

	go s.pollForEntries()

	return s, nil
}

func (m *mdnsService) Close() error {
	return m.server.Shutdown()
}

func (m *mdnsService) pollForEntries() {
	ticker := time.NewTicker(m.interval)
	for {
		select {
		case <-ticker.C:
			entriesCh := make(chan *mdns.ServiceEntry, 16)
			go func() {
				for entry := range entriesCh {
					m.handleEntry(entry)
				}
			}()

			qp := mdns.QueryParam{}
			qp.Domain = "local"
			qp.Entries = entriesCh
			qp.Service = ServiceTag
			qp.Timeout = time.Second * 5

			err := mdns.Query(&qp)
			if err != nil {
				log.Error("mdns lookup error: ", err)
			}
			close(entriesCh)
		}
	}
}

func (m *mdnsService) handleEntry(e *mdns.ServiceEntry) {
	mpeer, err := peer.IDB58Decode(e.Info)
	if err != nil {
		log.Warning("Error parsing peer ID from mdns entry: ", err)
		return
	}

	if mpeer == m.host.ID() {
		return
	}

	maddr, err := manet.FromNetAddr(&net.TCPAddr{
		IP:   e.AddrV4,
		Port: e.Port,
	})
	if err != nil {
		log.Warning("Error parsing multiaddr from mdns entry: ", err)
		return
	}

	pi := peer.PeerInfo{
		ID:    mpeer,
		Addrs: []ma.Multiaddr{maddr},
	}

	m.lk.Lock()
	for _, n := range m.notifees {
		n.HandlePeerFound(pi)
	}
	m.lk.Unlock()
}

func (m *mdnsService) RegisterNotifee(n Notifee) {
	m.lk.Lock()
	m.notifees = append(m.notifees, n)
	m.lk.Unlock()
}

func (m *mdnsService) UnregisterNotifee(n Notifee) {
	m.lk.Lock()
	found := -1
	for i, notif := range m.notifees {
		if notif == n {
			found = i
			break
		}
	}
	if found != -1 {
		m.notifees = append(m.notifees[:found], m.notifees[found+1:]...)
	}
	m.lk.Unlock()
}
