package identify

import (
	"fmt"
	"sync"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"

	config "github.com/jbenet/go-ipfs/config"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"

	host "github.com/jbenet/go-ipfs/p2p/host"
	inet "github.com/jbenet/go-ipfs/p2p/net"
	protocol "github.com/jbenet/go-ipfs/p2p/protocol"

	pb "github.com/jbenet/go-ipfs/p2p/protocol/identify/pb"
)

var log = eventlog.Logger("net/identify")

// ID is the protocol.ID of the Identify Service.
const ID protocol.ID = "/ipfs/identify"

// IpfsVersion holds the current protocol version for a client running this code
var IpfsVersion *semver.Version
var ClientVersion = "go-ipfs/" + config.CurrentVersionNumber

func init() {
	var err error
	IpfsVersion, err = semver.NewVersion("0.0.1")
	if err != nil {
		panic(fmt.Errorf("invalid protocol version: %v", err))
	}
}

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
}

func NewIDService(h host.Host) *IDService {
	s := &IDService{
		Host:   h,
		currid: make(map[inet.Conn]chan struct{}),
	}
	h.SetStreamHandler(ID, s.RequestHandler)
	return s
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
		log.Error("error opening initial stream for %s", ID)
		log.Event(context.TODO(), "IdentifyOpenFailed", c.RemotePeer())
	} else {

		// ok give the response to our handler.
		if err := protocol.WriteHeader(s, ID); err != nil {
			log.Error("error writing stream header for %s", ID)
			log.Event(context.TODO(), "IdentifyOpenFailed", c.RemotePeer())
		}
		ids.ResponseHandler(s)
	}

	ids.currmu.Lock()
	ch, found := ids.currid[c]
	delete(ids.currid, c)
	ids.currmu.Unlock()

	if !found {
		log.Errorf("IdentifyConn failed to find channel (programmer error) for %s", c)
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
		log.Errorf("%s error receiving message from %s %s", ID,
			c.RemotePeer(), c.RemoteMultiaddr())
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

	// set listen addrs
	laddrs, err := ids.Host.Network().InterfaceListenAddresses()
	if err != nil {
		log.Error(err)
	} else {
		mes.ListenAddrs = make([][]byte, len(laddrs))
		for i, addr := range laddrs {
			mes.ListenAddrs[i] = addr.Bytes()
		}
		log.Debugf("%s sent listen addrs to %s: %s", c.LocalPeer(), c.RemotePeer(), laddrs)
	}

	// set protocol versions
	s := IpfsVersion.String()
	mes.ProtocolVersion = &s
	mes.AgentVersion = &ClientVersion
}

func (ids *IDService) consumeMessage(mes *pb.Identify, c inet.Conn) {
	p := c.RemotePeer()

	// mes.Protocols
	// mes.ObservedAddr

	// mes.ListenAddrs
	laddrs := mes.GetListenAddrs()
	lmaddrs := make([]ma.Multiaddr, 0, len(laddrs))
	for _, addr := range laddrs {
		maddr, err := ma.NewMultiaddrBytes(addr)
		if err != nil {
			log.Errorf("%s failed to parse multiaddr from %s %s", ID,
				p, c.RemoteMultiaddr())
			continue
		}
		lmaddrs = append(lmaddrs, maddr)
	}

	// update our peerstore with the addresses.
	ids.Host.Peerstore().AddAddresses(p, lmaddrs)
	log.Debugf("%s received listen addrs for %s: %s", c.LocalPeer(), c.RemotePeer(), lmaddrs)

	// get protocol versions
	pv := *mes.ProtocolVersion
	av := *mes.AgentVersion
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
