package diagnostics

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"crypto/rand"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	pb "github.com/jbenet/go-ipfs/diagnostics/internal/pb"
	net "github.com/jbenet/go-ipfs/net"
	msg "github.com/jbenet/go-ipfs/net/message"
	peer "github.com/jbenet/go-ipfs/peer"
	util "github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("diagnostics")

const ResponseTimeout = time.Second * 10

// Diagnostics is a net service that manages requesting and responding to diagnostic
// requests
type Diagnostics struct {
	network net.Network
	sender  net.Sender
	self    peer.Peer

	diagLock sync.Mutex
	diagMap  map[string]time.Time
	birth    time.Time
}

// NewDiagnostics instantiates a new diagnostics service running on the given network
func NewDiagnostics(self peer.Peer, inet net.Network, sender net.Sender) *Diagnostics {
	return &Diagnostics{
		network: inet,
		sender:  sender,
		self:    self,
		diagMap: make(map[string]time.Time),
		birth:   time.Now(),
	}
}

type connDiagInfo struct {
	Latency time.Duration
	ID      string
}

type DiagInfo struct {
	// This nodes ID
	ID string

	// A list of peers this node currently has open connections to
	Connections []connDiagInfo

	// A list of keys provided by this node
	//    (currently not filled)
	Keys []string

	// How long this node has been running for
	LifeSpan time.Duration

	// Incoming Bandwidth Usage
	BwIn uint64

	// Outgoing Bandwidth Usage
	BwOut uint64

	// Information about the version of code this node is running
	CodeVersion string
}

// Marshal to json
func (di *DiagInfo) Marshal() []byte {
	b, err := json.Marshal(di)
	if err != nil {
		panic(err)
	}
	//TODO: also consider compressing this. There will be a lot of these
	return b
}

func (d *Diagnostics) getPeers() []peer.Peer {
	return d.network.GetPeerList()
}

func (d *Diagnostics) getDiagInfo() *DiagInfo {
	di := new(DiagInfo)
	di.CodeVersion = "github.com/jbenet/go-ipfs"
	di.ID = d.self.ID().Pretty()
	di.LifeSpan = time.Since(d.birth)
	di.Keys = nil // Currently no way to query datastore
	di.BwIn, di.BwOut = d.network.GetBandwidthTotals()

	for _, p := range d.getPeers() {
		d := connDiagInfo{p.GetLatency(), p.ID().Pretty()}
		di.Connections = append(di.Connections, d)
	}
	return di
}

func newID() string {
	id := make([]byte, 4)
	rand.Read(id)
	return string(id)
}

// GetDiagnostic runs a diagnostics request across the entire network
func (d *Diagnostics) GetDiagnostic(timeout time.Duration) ([]*DiagInfo, error) {
	log.Debug("Getting diagnostic.")
	ctx, _ := context.WithTimeout(context.TODO(), timeout)

	diagID := newID()
	d.diagLock.Lock()
	d.diagMap[diagID] = time.Now()
	d.diagLock.Unlock()

	log.Debug("Begin Diagnostic")

	peers := d.getPeers()
	log.Debugf("Sending diagnostic request to %d peers.", len(peers))

	var out []*DiagInfo
	di := d.getDiagInfo()
	out = append(out, di)

	pmes := newMessage(diagID)

	respdata := make(chan []byte)
	sends := 0
	for _, p := range peers {
		log.Debugf("Sending getDiagnostic to: %s", p)
		sends++
		go func(p peer.Peer) {
			data, err := d.getDiagnosticFromPeer(ctx, p, pmes)
			if err != nil {
				log.Errorf("GetDiagnostic error: %v", err)
				respdata <- nil
				return
			}
			respdata <- data
		}(p)
	}

	for i := 0; i < sends; i++ {
		data := <-respdata
		if data == nil {
			continue
		}
		out = appendDiagnostics(data, out)
	}
	return out, nil
}

func appendDiagnostics(data []byte, cur []*DiagInfo) []*DiagInfo {
	buf := bytes.NewBuffer(data)
	dec := json.NewDecoder(buf)
	for {
		di := new(DiagInfo)
		err := dec.Decode(di)
		if err != nil {
			if err != io.EOF {
				log.Errorf("error decoding DiagInfo: %v", err)
			}
			break
		}
		cur = append(cur, di)
	}
	return cur
}

// TODO: this method no longer needed.
func (d *Diagnostics) getDiagnosticFromPeer(ctx context.Context, p peer.Peer, mes *pb.Message) ([]byte, error) {
	rpmes, err := d.sendRequest(ctx, p, mes)
	if err != nil {
		return nil, err
	}
	return rpmes.GetData(), nil
}

func newMessage(diagID string) *pb.Message {
	pmes := new(pb.Message)
	pmes.DiagID = proto.String(diagID)
	return pmes
}

func (d *Diagnostics) sendRequest(ctx context.Context, p peer.Peer, pmes *pb.Message) (*pb.Message, error) {

	mes, err := msg.FromObject(p, pmes)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	rmes, err := d.sender.SendRequest(ctx, mes)
	if err != nil {
		return nil, err
	}
	if rmes == nil {
		return nil, errors.New("no response to request")
	}

	rtt := time.Since(start)
	log.Infof("diagnostic request took: %s", rtt.String())

	rpmes := new(pb.Message)
	if err := proto.Unmarshal(rmes.Data(), rpmes); err != nil {
		return nil, err
	}

	return rpmes, nil
}

func (d *Diagnostics) handleDiagnostic(p peer.Peer, pmes *pb.Message) (*pb.Message, error) {
	log.Debugf("HandleDiagnostic from %s for id = %s", p, pmes.GetDiagID())
	resp := newMessage(pmes.GetDiagID())

	// Make sure we havent already handled this request to prevent loops
	d.diagLock.Lock()
	_, found := d.diagMap[pmes.GetDiagID()]
	if found {
		d.diagLock.Unlock()
		return resp, nil
	}
	d.diagMap[pmes.GetDiagID()] = time.Now()
	d.diagLock.Unlock()

	buf := new(bytes.Buffer)
	di := d.getDiagInfo()
	buf.Write(di.Marshal())

	ctx, _ := context.WithTimeout(context.TODO(), ResponseTimeout)

	respdata := make(chan []byte)
	sendcount := 0
	for _, p := range d.getPeers() {
		log.Debugf("Sending diagnostic request to peer: %s", p)
		sendcount++
		go func(p peer.Peer) {
			out, err := d.getDiagnosticFromPeer(ctx, p, pmes)
			if err != nil {
				log.Errorf("getDiagnostic error: %v", err)
				respdata <- nil
				return
			}
			respdata <- out
		}(p)
	}

	for i := 0; i < sendcount; i++ {
		out := <-respdata
		_, err := buf.Write(out)
		if err != nil {
			log.Errorf("getDiagnostic write output error: %v", err)
			continue
		}
	}

	resp.Data = buf.Bytes()
	return resp, nil
}

func (d *Diagnostics) HandleMessage(ctx context.Context, mes msg.NetMessage) msg.NetMessage {
	mData := mes.Data()
	if mData == nil {
		log.Error("message did not include Data")
		return nil
	}

	mPeer := mes.Peer()
	if mPeer == nil {
		log.Error("message did not include a Peer")
		return nil
	}

	// deserialize msg
	pmes := new(pb.Message)
	err := proto.Unmarshal(mData, pmes)
	if err != nil {
		log.Errorf("Failed to decode protobuf message: %v", err)
		return nil
	}

	// Print out diagnostic
	log.Infof("[peer: %s] Got message from [%s]\n",
		d.self.ID().Pretty(), mPeer.ID().Pretty())

	// dispatch handler.
	rpmes, err := d.handleDiagnostic(mPeer, pmes)
	if err != nil {
		log.Errorf("handleDiagnostic error: %s", err)
		return nil
	}

	// if nil response, return it before serializing
	if rpmes == nil {
		return nil
	}

	// serialize response msg
	rmes, err := msg.FromObject(mPeer, rpmes)
	if err != nil {
		log.Errorf("Failed to encode protobuf message: %v", err)
		return nil
	}

	return rmes
}
