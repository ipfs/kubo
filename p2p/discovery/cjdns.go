package discovery

import (
	"io"
	"net"
	"sync"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	"github.com/ipfs/go-ipfs/p2p/host"
	swarm "github.com/ipfs/go-ipfs/p2p/net/swarm"

	"github.com/ehmry/go-cjdns/admin"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

type cjdnsService struct {
	admin    *admin.Conn
	host     host.Host
	lk       sync.Mutex
	notifees []Notifee
	interval time.Duration
}

func NewCjdnsService(host host.Host, interval time.Duration) (Service, error) {
	cjdnsConfig := &admin.CjdnsAdminConfig{
		Addr:     "127.0.0.1",
		Port:     11234,
		Password: "NONE",
	}
	admin, err := admin.Connect(cjdnsConfig)
	if err != nil {
		log.Error("cjdns connect error: ", err)
		return nil, err
	}

	log.Debug("cjdns admin api connected")

	service := &cjdnsService{
		admin:    admin,
		host:     host,
		interval: interval,
	}

	go func() {
		for {
			service.pollPeerStats()
			log.Fatal("quit here")
		}
	}()

	return service, nil
}

func (cjdns *cjdnsService) Close() error {
	return nil
}

func (cjdns *cjdnsService) pollPeerStats() {
	peerstats, err := cjdns.admin.InterfaceController_peerStats()
	if err != nil {
		log.Errorf("cjdns peerstats error: %s", err)
		return
	}

	for _, peer := range peerstats {
		ipaddr := peer.PublicKey.IP()
		maddr, err := manet.FromNetAddr(&net.TCPAddr{IP: ipaddr, Port: 4001})
		if err != nil {
			log.Errorf("corrupt multiaddr: [%s] %s", ipaddr, err)
			continue
		}

		p2pnet := cjdns.host.Network()
		swnet := p2pnet.(*swarm.Network)
		conn, err := swnet.Swarm().Dialer().Dial(context.TODO(), maddr, "")
		if err != nil {
			log.Debugf("dial failed: [%s] %s", maddr.String(), err)
			continue
		}

		rp := conn.RemotePeer().Pretty()
		if len(rp) == 0 {
			log.Errorf("handshake failed with %s", maddr.String())
			continue
		}
		maddr = ma.NewMultiaddr(maddr.String() + "/ipfs/" + rp)
		log.Debugf("possible cjdns peer: %s", maddr)
	}
}

func (c *cjdnsService) RegisterNotifee(n Notifee) {
	c.lk.Lock()
	c.notifees = append(c.notifees, n)
	c.lk.Unlock()
}

func (c *cjdnsService) UnregisterNotifee(n Notifee) {
	c.lk.Lock()
	found := -1
	for i, notif := range c.notifees {
		if notif == n {
			found = i
			break
		}
	}
	if found != -1 {
		c.notifees = append(c.notifees[:found], c.notifees[found+1:]...)
	}
	c.lk.Unlock()
}
