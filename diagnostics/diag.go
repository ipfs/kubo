// package diagnostics implements a network diagnostics service that
// allows a request to traverse the network and gather information
// on every node connected to it.
package diagnostics

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	context "context"
	pb "github.com/ipfs/go-ipfs/diagnostics/pb"
	host "gx/ipfs/QmPsRtodRuBUir32nz5v4zuSBTSszrR1d3fA6Ahb6eaejj/go-libp2p-host"
	inet "gx/ipfs/QmQx1dHDDYENugYgqA22BaBrRfuv1coSsuPiM7rYh1wwGH/go-libp2p-net"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	ctxio "gx/ipfs/QmTKsRYeY4simJyf37K93juSq75Lo8MVCDJ7owjmf46u8W/go-context/io"
	ggio "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/io"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	protocol "gx/ipfs/QmZNkThpqfVXs9GNbexPrfBbXSLNYeKrE7jwFM2oqHbyqN/go-libp2p-protocol"
	peer "gx/ipfs/QmfMmLGoKzCHDN7cGgk64PJr4iipzidDRME8HABSJqvmhC/go-libp2p-peer"
)

var log = logging.Logger("diagnostics")

// ProtocolDiag is the diagnostics protocol.ID
var ProtocolDiag protocol.ID = "/ipfs/diag/net/1.0.0"
var ProtocolDiagOld protocol.ID = "/ipfs/diagnostics"

var ErrAlreadyRunning = errors.New("diagnostic with that ID already running")

const ResponseTimeout = time.Second * 10
const HopTimeoutDecrement = time.Second * 2

// Diagnostics is a net service that manages requesting and responding to diagnostic
// requests
type Diagnostics struct {
	host host.Host
	self peer.ID

	diagLock sync.Mutex
	diagMap  map[string]time.Time
	birth    time.Time
}

// NewDiagnostics instantiates a new diagnostics service running on the given network
func NewDiagnostics(self peer.ID, h host.Host) *Diagnostics {
	d := &Diagnostics{
		host:    h,
		self:    self,
		birth:   time.Now(),
		diagMap: make(map[string]time.Time),
	}

	h.SetStreamHandler(ProtocolDiag, d.handleNewStream)
	h.SetStreamHandler(ProtocolDiagOld, d.handleNewStream)
	return d
}

type connDiagInfo struct {
	Latency time.Duration
	ID      string
	Count   int
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
	// TODO rename Uptime
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

func (d *Diagnostics) getPeers() map[peer.ID]int {
	counts := make(map[peer.ID]int)
	for _, p := range d.host.Network().Peers() {
		counts[p]++
	}

	return counts
}

func (d *Diagnostics) getDiagInfo() *DiagInfo {
	di := new(DiagInfo)
	di.CodeVersion = "github.com/ipfs/go-ipfs"
	di.ID = d.self.Pretty()
	di.LifeSpan = time.Since(d.birth)
	di.Keys = nil // Currently no way to query datastore

	// di.BwIn, di.BwOut = d.host.BandwidthTotals() //TODO fix this.

	for p, n := range d.getPeers() {
		d := connDiagInfo{
			Latency: d.host.Peerstore().LatencyEWMA(p),
			ID:      p.Pretty(),
			Count:   n,
		}
		di.Connections = append(di.Connections, d)
	}
	return di
}

func newID() string {
	id := make([]byte, 16)
	rand.Read(id)
	return string(id)
}

// GetDiagnostic runs a diagnostics request across the entire network
func (d *Diagnostics) GetDiagnostic(ctx context.Context, timeout time.Duration) ([]*DiagInfo, error) {
	log.Debug("getting diagnostic")
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	diagID := newID()
	d.diagLock.Lock()
	d.diagMap[diagID] = time.Now()
	d.diagLock.Unlock()

	log.Debug("begin diagnostic")

	peers := d.getPeers()
	log.Debugf("Sending diagnostic request to %d peers.", len(peers))

	pmes := newMessage(diagID)

	pmes.SetTimeoutDuration(timeout - HopTimeoutDecrement) // decrease timeout per hop
	dpeers, err := d.getDiagnosticFromPeers(ctx, d.getPeers(), pmes)
	if err != nil {
		return nil, fmt.Errorf("diagnostic from peers err: %s", err)
	}

	di := d.getDiagInfo()
	out := []*DiagInfo{di}
	for dpi := range dpeers {
		out = append(out, dpi)
	}
	return out, nil
}

func decodeDiagJson(data []byte) (*DiagInfo, error) {
	di := new(DiagInfo)
	err := json.Unmarshal(data, di)
	if err != nil {
		return nil, err
	}

	return di, nil
}

func (d *Diagnostics) getDiagnosticFromPeers(ctx context.Context, peers map[peer.ID]int, pmes *pb.Message) (<-chan *DiagInfo, error) {
	respdata := make(chan *DiagInfo)
	wg := sync.WaitGroup{}
	for p := range peers {
		wg.Add(1)
		log.Debugf("Sending diagnostic request to peer: %s", p)
		go func(p peer.ID) {
			defer wg.Done()
			out, err := d.getDiagnosticFromPeer(ctx, p, pmes)
			if err != nil {
				log.Debugf("Error getting diagnostic from %s: %s", p, err)
				return
			}
			for d := range out {
				select {
				case respdata <- d:
				case <-ctx.Done():
					return
				}
			}
		}(p)
	}

	go func() {
		wg.Wait()
		close(respdata)
	}()

	return respdata, nil
}

func (d *Diagnostics) getDiagnosticFromPeer(ctx context.Context, p peer.ID, pmes *pb.Message) (<-chan *DiagInfo, error) {
	s, err := d.host.NewStream(ctx, p, ProtocolDiag, ProtocolDiagOld)
	if err != nil {
		return nil, err
	}

	cr := ctxio.NewReader(ctx, s) // ok to use. we defer close stream in this func
	cw := ctxio.NewWriter(ctx, s) // ok to use. we defer close stream in this func
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	w := ggio.NewDelimitedWriter(cw)

	start := time.Now()

	if err := w.WriteMsg(pmes); err != nil {
		return nil, err
	}

	out := make(chan *DiagInfo)
	go func() {

		defer func() {
			close(out)
			s.Close()
			rtt := time.Since(start)
			log.Infof("diagnostic request took: %s", rtt.String())
		}()

		for {
			rpmes := new(pb.Message)
			if err := r.ReadMsg(rpmes); err != nil {
				log.Debugf("Error reading diagnostic from stream: %s", err)
				return
			}
			if rpmes == nil {
				log.Debug("got no response back from diag request")
				return
			}

			di, err := decodeDiagJson(rpmes.GetData())
			if err != nil {
				log.Debug(err)
				return
			}

			select {
			case out <- di:
			case <-ctx.Done():
				return
			}
		}

	}()

	return out, nil
}

func newMessage(diagID string) *pb.Message {
	pmes := new(pb.Message)
	pmes.DiagID = proto.String(diagID)
	return pmes
}

func (d *Diagnostics) HandleMessage(ctx context.Context, s inet.Stream) error {

	cr := ctxio.NewReader(ctx, s)
	cw := ctxio.NewWriter(ctx, s)
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax) // maxsize
	w := ggio.NewDelimitedWriter(cw)

	// deserialize msg
	pmes := new(pb.Message)
	if err := r.ReadMsg(pmes); err != nil {
		log.Debugf("Failed to decode protobuf message: %v", err)
		return nil
	}

	// Print out diagnostic
	log.Infof("[peer: %s] Got message from [%s]\n",
		d.self.Pretty(), s.Conn().RemotePeer())

	// Make sure we havent already handled this request to prevent loops
	if err := d.startDiag(pmes.GetDiagID()); err != nil {
		return nil
	}

	resp := newMessage(pmes.GetDiagID())
	resp.Data = d.getDiagInfo().Marshal()
	if err := w.WriteMsg(resp); err != nil {
		log.Debugf("Failed to write protobuf message over stream: %s", err)
		return err
	}

	timeout := pmes.GetTimeoutDuration()
	if timeout < HopTimeoutDecrement {
		return fmt.Errorf("timeout too short: %s", timeout)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	pmes.SetTimeoutDuration(timeout - HopTimeoutDecrement)

	dpeers, err := d.getDiagnosticFromPeers(ctx, d.getPeers(), pmes)
	if err != nil {
		log.Debugf("diagnostic from peers err: %s", err)
		return err
	}
	for b := range dpeers {
		resp := newMessage(pmes.GetDiagID())
		resp.Data = b.Marshal()
		if err := w.WriteMsg(resp); err != nil {
			log.Debugf("Failed to write protobuf message over stream: %s", err)
			return err
		}
	}

	return nil
}

func (d *Diagnostics) startDiag(id string) error {
	d.diagLock.Lock()
	_, found := d.diagMap[id]
	if found {
		d.diagLock.Unlock()
		return ErrAlreadyRunning
	}
	d.diagMap[id] = time.Now()
	d.diagLock.Unlock()
	return nil
}

func (d *Diagnostics) handleNewStream(s inet.Stream) {
	d.HandleMessage(context.Background(), s)
	s.Close()
}
