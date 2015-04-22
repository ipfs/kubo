package discovery

import (
	"fmt"
	"io"
	"io/ioutil"
	golog "log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	"github.com/ipfs/go-ipfs/p2p/host"
	"github.com/ipfs/go-ipfs/p2p/peer"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("mdns")

const LookupFrequency = time.Second * 5
const ServiceTag = "discovery.ipfs.io"

type Service interface {
	io.Closer
	RegisterNotifee(Notifee)
	UnregisterNotifee(Notifee)
}

type Notifee interface {
	HandlePeerFound(peer.PeerInfo)
}

type mdnsService struct {
	server  *mdns.Server
	service *mdns.MDNSService
	host    host.Host

	lk       sync.Mutex
	notifees []Notifee
}

func NewMdnsService(peerhost host.Host) (Service, error) {

	// TODO: dont let mdns use logging...
	golog.SetOutput(ioutil.Discard)

	// determine my local swarm port
	port := 4001
	for _, addr := range peerhost.Addrs() {
		parts := strings.Split(addr.String(), "/")
		fmt.Println("parts len: ", len(parts))
		if len(parts) == 5 && parts[3] == "tcp" {
			n, err := strconv.Atoi(parts[4])
			if err != nil {
				return nil, err
			}
			port = n
			break
		}
	}
	fmt.Println("using port: ", port)

	myid := peerhost.ID().Pretty()

	info := []string{myid}
	service, err := mdns.NewMDNSService(myid, ServiceTag, "", "", port, nil, info)
	if err != nil {
		return nil, err
	}

	// Create the mDNS server, defer shutdown
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, err
	}

	s := &mdnsService{
		server:  server,
		service: service,
		host:    peerhost,
	}

	go s.pollForEntries()

	return s, nil
}

func (m *mdnsService) Close() error {
	return m.server.Shutdown()
}

func (m *mdnsService) pollForEntries() {
	ticker := time.NewTicker(LookupFrequency)
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
			qp.Timeout = time.Second * 3

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
