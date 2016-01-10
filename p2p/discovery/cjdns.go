package discovery

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	host "github.com/ipfs/go-ipfs/p2p/host"
	swarm "github.com/ipfs/go-ipfs/p2p/net/swarm"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	// TODO: kill this dependency
	config "github.com/ipfs/go-ipfs/repo/config"

	cjdns "github.com/ehmry/go-cjdns/admin"
)

type cjdnsService struct {
	host     host.Host
	dialed   map[ma.Multiaddr]bool
	ctx      context.Context
	lk       sync.Mutex
	notifees []Notifee
}

func NewCjdnsService(ctx context.Context, host host.Host, cfg config.Cjdns) (Service, error) {
	s := &cjdnsService{
		host:   host,
		dialed: map[ma.Multiaddr]bool{},
		ctx:    ctx,
	}

	admin, err := cjdnsAdmin(cfg)
	if err != nil {
		log.Errorf("cjdns admin error: %s", err)
	} else {
		s.Discover(admin)
	}

	go func() {
		s.Discover(admin)
		ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
		rticker := time.NewTicker(time.Duration(cfg.RefreshInterval) * time.Second)
		select {
		case <-ticker.C:
			admin, err := cjdnsAdmin(cfg)
			if err != nil {
				log.Errorf("cjdns admin error: %s", err)
			} else {
				s.Discover(admin)
			}
		case <-rticker.C:
			s.dialed = map[ma.Multiaddr]bool{}
		case <-s.ctx.Done():
			ticker.Stop()
			rticker.Stop()
			return
		}
	}()

	return s, nil
}

// TODO is this right?
func (s *cjdnsService) Close() error {
	s.ctx.Done()
	return nil
}

func (s *cjdnsService) Discover(admin *cjdns.Conn) {
	nodes, err := knownCjdnsNodes(admin)
	if err != nil {
		log.Errorf("known cjdns nodes error: %s", err)
		return
	}

	for _, maddr := range nodes {
		if _, dialed := s.dialed[maddr]; dialed {
			continue
		}
		id, err := s.dial(maddr)
		if err != nil {
			log.Debugf("dial error: %s", err)
			continue
		}

		str := maddr.String() + "/ipfs/" + id.Pretty()
		maddrid, err := ma.NewMultiaddr(str)
		if err != nil {
			log.Errorf("multiaddr error: [%s] %s", str, err)
			continue
		}

		s.dialed[maddr] = true
		log.Infof("discovered %s", str)
		s.emit(id, maddrid)
	}
}

func (s *cjdnsService) dial(maddr ma.Multiaddr) (peer.ID, error) {
	p2pnet := s.host.Network()
	swnet := p2pnet.(*swarm.Network)
	conn, err := swnet.Swarm().Dialer().Dial(s.ctx, maddr, "")
	if err != nil {
		return "", err
	}

	id := conn.RemotePeer()
	if len(id) == 0 {
		return "", fmt.Debugf("handshake failed with %s", maddr.String())
	}

	return id, nil
}

func (s *cjdnsService) emit(id peer.ID, maddr ma.Multiaddr) {
	pi := peer.PeerInfo{
		ID:    id,
		Addrs: []ma.Multiaddr{maddr},
	}

	s.lk.Lock()
	for _, n := range s.notifees {
		n.HandlePeerFound(pi)
	}
	s.lk.Unlock()
}

func knownCjdnsNodes(admin *cjdns.Conn) ([]ma.Multiaddr, error) {
	nodes := []ma.Multiaddr{}

	peers, err := admin.InterfaceController_peerStats()
	if err != nil {
		return nil, err
	}
	for _, peer := range peers {
		maddr, err := fromCjdnsIP(peer.PublicKey.IP())
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, maddr)
	}

	nodestore, err := admin.NodeStore_dumpTable()
	if err != nil {
		return nil, err
	}
	for _, node := range nodestore {
		maddr, err := fromCjdnsIP(*node.IP)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, maddr)
	}

	return nodes, nil
}

func fromCjdnsIP(ip net.IP) (ma.Multiaddr, error) {
	return manet.FromNetAddr(&net.TCPAddr{IP: ip, Port: 4001})
}

func cjdnsAdmin(cfg config.Cjdns) (*cjdns.Conn, error) {
	maddr, err := ma.NewMultiaddr(cfg.AdminAddress)
	if err != nil {
		panic(fmt.Errorf("invalid Cjdns.AdminAddress: %s", err))
	}
	p := strings.Split(maddr.String(), "/")[1:]
	if p[2] != "udp" {
		panic(fmt.Errorf("non-udp Cjdns.AdminAddress: %s", p[2]))
	}

	port, _ := strconv.ParseInt(p[3], 10, 16)
	c := &cjdns.CjdnsAdminConfig{
		Addr:     p[1],
		Port:     int(port),
		Password: "NONE",
	}
	admin, err := cjdns.Connect(c)
	if err != nil {
		return nil, err
	}
	return admin, nil
}

func (s *cjdnsService) RegisterNotifee(n Notifee) {
	s.lk.Lock()
	s.notifees = append(s.notifees, n)
	s.lk.Unlock()
}

func (s *cjdnsService) UnregisterNotifee(n Notifee) {
	s.lk.Lock()
	found := -1
	for i, notif := range s.notifees {
		if notif == n {
			found = i
			break
		}
	}
	if found != -1 {
		s.notifees = append(s.notifees[:found], s.notifees[found+1:]...)
	}
	s.lk.Unlock()
}
