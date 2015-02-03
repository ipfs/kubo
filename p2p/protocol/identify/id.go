package identify

import (
	"strings"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"
	pb "github.com/jbenet/go-ipfs/p2p/protocol/identify/pb"
	config "github.com/jbenet/go-ipfs/repo/config"
	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
)

var log = eventlog.Logger("net/identify")

// ID is the protocol.ID of the Identify Service.
const ID protocol.ID = "/ipfs/identify"

// IpfsVersion holds the current protocol version for a client running this code
// TODO(jbenet): fix the versioning mess.
const IpfsVersion = "ipfs/0.1.0"
const ClientVersion = "go-ipfs/" + config.CurrentVersionNumber

// IDService is a structure that implements ProtocolIdentify.
// It is a trivial service that gives the other peer some
// useful information about the local peer. A sort of hello.
//
// The IDService sends:
//  * Our IPFS Protocol Version
//  * Our IPFS Agent Version
//  * Our public Listen Addresses
type IDService struct {
	Host host.Host

	// connections undergoing identification
	// for wait purposes
	currid map[inet.Conn]chan struct{}
	currmu sync.RWMutex

	// our own observed addresses.
	// TODO: instead of expiring, remove these when we disconnect
	addrs peer.AddrManager
}

func NewIDService(h host.Host) *IDService {
	s := &IDService{
		Host:   h,
		currid: make(map[inet.Conn]chan struct{}),
	}
	h.SetStreamHandler(ID, s.RequestHandler)
	return s
}

// OwnObservedAddrs returns the addresses peers have reported we've dialed from
func (ids *IDService) OwnObservedAddrs() []ma.Multiaddr {
	return ids.addrs.Addrs(ids.Host.ID())
}

func (ids *IDService) IdentifyConn(c inet.Conn) {
	ids.currmu.Lock()
	if wait, found := ids.currid[c]; found {
		ids.currmu.Unlock()
		log.Debugf("IdentifyConn called twice on: %s", c)
		<-wait // already identifying it. wait for it.
		return
	}
	ids.currid[c] = make(chan struct{})
	ids.currmu.Unlock()

	s, err := c.NewStream()
	if err != nil {
		log.Debugf("error opening initial stream for %s", ID)
		log.Event(context.TODO(), "IdentifyOpenFailed", c.RemotePeer())
	} else {

		// ok give the response to our handler.
		if err := protocol.WriteHeader(s, ID); err != nil {
			log.Debugf("error writing stream header for %s", ID)
			log.Event(context.TODO(), "IdentifyOpenFailed", c.RemotePeer())
		}
		ids.ResponseHandler(s)
	}

	ids.currmu.Lock()
	ch, found := ids.currid[c]
	delete(ids.currid, c)
	ids.currmu.Unlock()

	if !found {
		log.Debugf("IdentifyConn failed to find channel (programmer error) for %s", c)
		return
	}

	close(ch) // release everyone waiting.
}

func (ids *IDService) RequestHandler(s inet.Stream) {
	defer s.Close()
	c := s.Conn()

	w := ggio.NewDelimitedWriter(s)
	mes := pb.Identify{}
	ids.populateMessage(&mes, s.Conn())
	w.WriteMsg(&mes)

	log.Debugf("%s sent message to %s %s", ID,
		c.RemotePeer(), c.RemoteMultiaddr())
}

func (ids *IDService) ResponseHandler(s inet.Stream) {
	defer s.Close()
	c := s.Conn()

	r := ggio.NewDelimitedReader(s, 2048)
	mes := pb.Identify{}
	if err := r.ReadMsg(&mes); err != nil {
		return
	}
	ids.consumeMessage(&mes, c)

	log.Debugf("%s received message from %s %s", ID,
		c.RemotePeer(), c.RemoteMultiaddr())
}

func (ids *IDService) populateMessage(mes *pb.Identify, c inet.Conn) {

	// set protocols this node is currently handling
	protos := ids.Host.Mux().Protocols()
	mes.Protocols = make([]string, len(protos))
	for i, p := range protos {
		mes.Protocols[i] = string(p)
	}

	// observed address so other side is informed of their
	// "public" address, at least in relation to us.
	mes.ObservedAddr = c.RemoteMultiaddr().Bytes()

	// set listen addrs, get our latest addrs from Host.
	laddrs := ids.Host.Addrs()
	mes.ListenAddrs = make([][]byte, len(laddrs))
	for i, addr := range laddrs {
		mes.ListenAddrs[i] = addr.Bytes()
	}
	log.Debugf("%s sent listen addrs to %s: %s", c.LocalPeer(), c.RemotePeer(), laddrs)

	// set protocol versions
	pv := IpfsVersion
	av := ClientVersion
	mes.ProtocolVersion = &pv
	mes.AgentVersion = &av
}

func (ids *IDService) consumeMessage(mes *pb.Identify, c inet.Conn) {
	p := c.RemotePeer()

	// mes.Protocols

	// mes.ObservedAddr
	ids.consumeObservedAddress(mes.GetObservedAddr(), c)

	// mes.ListenAddrs
	laddrs := mes.GetListenAddrs()
	lmaddrs := make([]ma.Multiaddr, 0, len(laddrs))
	for _, addr := range laddrs {
		maddr, err := ma.NewMultiaddrBytes(addr)
		if err != nil {
			log.Debugf("%s failed to parse multiaddr from %s %s", ID,
				p, c.RemoteMultiaddr())
			continue
		}
		lmaddrs = append(lmaddrs, maddr)
	}

	// update our peerstore with the addresses. here, we SET the addresses, clearing old ones.
	// We are receiving from the peer itself. this is current address ground truth.
	ids.Host.Peerstore().SetAddrs(p, lmaddrs, peer.ConnectedAddrTTL)
	log.Debugf("%s received listen addrs for %s: %s", c.LocalPeer(), c.RemotePeer(), lmaddrs)

	// get protocol versions
	pv := mes.GetProtocolVersion()
	av := mes.GetAgentVersion()

	// version check. if we shouldn't talk, bail.
	// TODO: at this point, we've already exchanged information.
	// move this into a first handshake before the connection can open streams.
	if !protocolVersionsAreCompatible(pv, IpfsVersion) {
		c.Close()
		return
	}

	ids.Host.Peerstore().Put(p, "ProtocolVersion", pv)
	ids.Host.Peerstore().Put(p, "AgentVersion", av)
}

// IdentifyWait returns a channel which will be closed once
// "ProtocolIdentify" (handshake3) finishes on given conn.
// This happens async so the connection can start to be used
// even if handshake3 knowledge is not necesary.
// Users **MUST** call IdentifyWait _after_ IdentifyConn
func (ids *IDService) IdentifyWait(c inet.Conn) <-chan struct{} {
	ids.currmu.Lock()
	ch, found := ids.currid[c]
	ids.currmu.Unlock()
	if found {
		return ch
	}

	// if not found, it means we are already done identifying it, or
	// haven't even started. either way, return a new channel closed.
	ch = make(chan struct{})
	close(ch)
	return ch
}

func (ids *IDService) consumeObservedAddress(observed []byte, c inet.Conn) {
	if observed == nil {
		return
	}

	maddr, err := ma.NewMultiaddrBytes(observed)
	if err != nil {
		log.Debugf("error parsing received observed addr for %s: %s", c, err)
		return
	}

	// we should only use ObservedAddr when our connection's LocalAddr is one
	// of our ListenAddrs. If we Dial out using an ephemeral addr, knowing that
	// address's external mapping is not very useful because the port will not be
	// the same as the listen addr.
	ifaceaddrs, err := ids.Host.Network().InterfaceListenAddresses()
	if err != nil {
		log.Infof("failed to get interface listen addrs", err)
		return
	}

	log.Debugf("identify identifying observed multiaddr: %s %s", c.LocalMultiaddr(), ifaceaddrs)
	if !addrInAddrs(c.LocalMultiaddr(), ifaceaddrs) {
		// not in our list
		return
	}

	// ok! we have the observed version of one of our ListenAddresses!
	log.Debugf("added own observed listen addr: %s --> %s", c.LocalMultiaddr(), maddr)
	ids.addrs.AddAddr(ids.Host.ID(), maddr, peer.OwnObservedAddrTTL)
}

func addrInAddrs(a ma.Multiaddr, as []ma.Multiaddr) bool {
	for _, b := range as {
		if a.Equal(b) {
			return true
		}
	}
	return false
}

// protocolVersionsAreCompatible checks that the two implementations
// can talk to each other. It will use semver, but for now while
// we're in tight development, we will return false for minor version
// changes too.
func protocolVersionsAreCompatible(v1, v2 string) bool {
	if strings.HasPrefix(v1, "ipfs/") {
		v1 = v1[5:]
	}
	if strings.HasPrefix(v2, "ipfs/") {
		v2 = v2[5:]
	}

	v1s, err := semver.NewVersion(v1)
	if err != nil {
		return false
	}

	v2s, err := semver.NewVersion(v2)
	if err != nil {
		return false
	}

	return v1s.Major == v2s.Major && v1s.Minor == v2s.Minor
}

// netNotifiee defines methods to be used with the IpfsDHT
type netNotifiee IDService

func (nn *netNotifiee) IDService() *IDService {
	return (*IDService)(nn)
}

func (nn *netNotifiee) Connected(n inet.Network, v inet.Conn) {
	// TODO: deprecate the setConnHandler hook, and kick off
	// identification here.
}

func (nn *netNotifiee) Disconnected(n inet.Network, v inet.Conn) {
	// undo the setting of addresses to peer.ConnectedAddrTTL we did
	ids := nn.IDService()
	ps := ids.Host.Peerstore()
	addrs := ps.Addrs(v.RemotePeer())
	ps.SetAddrs(v.RemotePeer(), addrs, peer.RecentlyConnectedAddrTTL)
}

func (nn *netNotifiee) OpenedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) ClosedStream(n inet.Network, v inet.Stream) {}
func (nn *netNotifiee) Listen(n inet.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n inet.Network, a ma.Multiaddr) {}
