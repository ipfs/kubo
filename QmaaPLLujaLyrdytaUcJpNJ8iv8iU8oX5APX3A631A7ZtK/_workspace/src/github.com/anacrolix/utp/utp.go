// Package utp implements uTP, the micro transport protocol as used with
// Bittorrent. It opts for simplicity and reliability over strict adherence to
// the (poor) spec. It allows using the underlying OS-level transport despite
// dispatching uTP on top to allow for example, shared socket use with DHT.
// Additionally, multiple uTP connections can share the same OS socket, to
// truly realize uTP's claim to be light on system and network switching
// resources.
//
// Socket is a wrapper of net.UDPConn, and performs dispatching of uTP packets
// to attached uTP Conns. Dial and Accept is done via Socket. Conn implements
// net.Conn over uTP, via aforementioned Socket.
package utp

import (
	"encoding/binary"
	"errors"
	"expvar"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/jitter"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/missinggo"
)

const (
	// Maximum received SYNs that haven't been accepted. If more SYNs are
	// received, a pseudo randomly selected SYN is replied to with a reset to
	// make room.
	backlog = 50

	// IPv6 min MTU is 1280, -40 for IPv6 header, and ~8 for fragment header?
	minMTU     = 1232
	recvWindow = 0x8000 // 32KiB
	// uTP header of 20, +2 for the next extension, and 8 bytes of selective
	// ACK.
	maxHeaderSize  = 30
	maxPayloadSize = minMTU - maxHeaderSize
	maxRecvSize    = 0x2000

	// Maximum out-of-order packets to buffer.
	maxUnackedInbound = 64

	// If an send isn't acknowledged after this period, its connection is
	// destroyed. There are resends during this period.
	sendTimeout = 15 * time.Second
)

var (
	ackSkippedResends = expvar.NewInt("utpAckSkippedResends")
	// Inbound packets processed by a Conn.
	deliveriesProcessed = expvar.NewInt("utpDeliveriesProcessed")
	sentStatePackets    = expvar.NewInt("utpSentStatePackets")
	unusedReads         = expvar.NewInt("utpUnusedReads")
	sendBufferPool      = sync.Pool{
		New: func() interface{} { return make([]byte, minMTU) },
	}
)

type deadlineCallback struct {
	deadline time.Time
	timer    *time.Timer
	callback func()
	inited   bool
}

func (me *deadlineCallback) deadlineExceeded() bool {
	return !me.deadline.IsZero() && !time.Now().Before(me.deadline)
}

func (me *deadlineCallback) updateTimer() {
	if me.timer != nil {
		me.timer.Stop()
	}
	if me.deadline.IsZero() {
		return
	}
	if me.callback == nil {
		panic("deadline callback is nil")
	}
	me.timer = time.AfterFunc(me.deadline.Sub(time.Now()), me.callback)
}

func (me *deadlineCallback) setDeadline(t time.Time) {
	me.deadline = t
	me.updateTimer()
}

func (me *deadlineCallback) setCallback(f func()) {
	me.callback = f
	me.updateTimer()
}

type connDeadlines struct {
	// mu          sync.Mutex
	read, write deadlineCallback
}

func (c *connDeadlines) SetDeadline(t time.Time) error {
	c.read.setDeadline(t)
	c.write.setDeadline(t)
	return nil
}

func (c *connDeadlines) SetReadDeadline(t time.Time) error {
	c.read.setDeadline(t)
	return nil
}

func (c *connDeadlines) SetWriteDeadline(t time.Time) error {
	c.write.setDeadline(t)
	return nil
}

// Strongly-type guarantee of resolved network address.
type resolvedAddrStr string

// Uniquely identifies any uTP connection on top of the underlying packet
// stream.
type connKey struct {
	remoteAddr resolvedAddrStr
	connID     uint16
}

// A Socket wraps a net.PacketConn, diverting uTP packets to its child uTP
// Conns.
type Socket struct {
	mu      sync.RWMutex
	event   sync.Cond
	pc      net.PacketConn
	conns   map[connKey]*Conn
	backlog map[syn]struct{}
	reads   chan read
	closing chan struct{}

	unusedReads chan read
	connDeadlines
	// If a read error occurs on the underlying net.PacketConn, it is put
	// here. This is because reading is done in its own goroutine to dispatch
	// to uTP Conns.
	ReadErr error
}

type read struct {
	data []byte
	from net.Addr
}

type syn struct {
	seq_nr, conn_id uint16
	addr            string
}

const (
	extensionTypeSelectiveAck = 1
)

type extensionField struct {
	Type  byte
	Bytes []byte
}

type header struct {
	Type          st
	Version       int
	ConnID        uint16
	Timestamp     uint32
	TimestampDiff uint32
	WndSize       uint32
	SeqNr         uint16
	AckNr         uint16
	Extensions    []extensionField
}

var (
	mu                         sync.RWMutex
	logLevel                   = 0
	artificialPacketDropChance = 0.0
)

func init() {
	logLevel, _ = strconv.Atoi(os.Getenv("GO_UTP_LOGGING"))
	fmt.Sscanf(os.Getenv("GO_UTP_PACKET_DROP"), "%f", &artificialPacketDropChance)
}

var (
	errClosed                   = errors.New("closed")
	errNotImplemented           = errors.New("not implemented")
	errTimeout        net.Error = timeoutError{"i/o timeout"}
	errAckTimeout               = timeoutError{"timed out waiting for ack"}
)

type timeoutError struct {
	msg string
}

func (me timeoutError) Timeout() bool   { return true }
func (me timeoutError) Error() string   { return me.msg }
func (me timeoutError) Temporary() bool { return false }

func unmarshalExtensions(_type byte, b []byte) (n int, ef []extensionField, err error) {
	for _type != 0 {
		if _type != extensionTypeSelectiveAck {
			// An extension type that is not known to us. Generally we're
			// unmarshalling an packet that isn't actually uTP but we don't
			// yet know for sure until we try to deliver it.

			// logonce.Stderr.Printf("utp extension %d", _type)
		}
		if len(b) < 2 || len(b) < int(b[1])+2 {
			err = fmt.Errorf("buffer ends prematurely: %x", b)
			return
		}
		ef = append(ef, extensionField{
			Type:  _type,
			Bytes: append([]byte{}, b[2:int(b[1])+2]...),
		})
		_type = b[0]
		n += 2 + int(b[1])
		b = b[2+int(b[1]):]
	}
	return
}

var errInvalidHeader = errors.New("invalid header")

func (h *header) Unmarshal(b []byte) (n int, err error) {
	h.Type = st(b[0] >> 4)
	h.Version = int(b[0] & 0xf)
	if h.Type > stMax || h.Version != 1 {
		err = errInvalidHeader
		return
	}
	n, h.Extensions, err = unmarshalExtensions(b[1], b[20:])
	if err != nil {
		return
	}
	h.ConnID = binary.BigEndian.Uint16(b[2:4])
	h.Timestamp = binary.BigEndian.Uint32(b[4:8])
	h.TimestampDiff = binary.BigEndian.Uint32(b[8:12])
	h.WndSize = binary.BigEndian.Uint32(b[12:16])
	h.SeqNr = binary.BigEndian.Uint16(b[16:18])
	h.AckNr = binary.BigEndian.Uint16(b[18:20])
	n += 20
	return
}

func (h *header) Marshal() (ret []byte) {
	hLen := 20 + func() (ret int) {
		for _, ext := range h.Extensions {
			ret += 2 + len(ext.Bytes)
		}
		return
	}()
	ret = sendBufferPool.Get().([]byte)[:hLen:minMTU]
	// ret = make([]byte, hLen, minMTU)
	p := ret // Used for manipulating ret.
	p[0] = byte(h.Type<<4 | 1)
	binary.BigEndian.PutUint16(p[2:4], h.ConnID)
	binary.BigEndian.PutUint32(p[4:8], h.Timestamp)
	binary.BigEndian.PutUint32(p[8:12], h.TimestampDiff)
	binary.BigEndian.PutUint32(p[12:16], h.WndSize)
	binary.BigEndian.PutUint16(p[16:18], h.SeqNr)
	binary.BigEndian.PutUint16(p[18:20], h.AckNr)
	// Pointer to the last type field so the next extension can set it.
	_type := &p[1]
	// We're done with the basic header.
	p = p[20:]
	for _, ext := range h.Extensions {
		*_type = ext.Type
		// The next extension's type will go here.
		_type = &p[0]
		p[1] = uint8(len(ext.Bytes))
		if int(p[1]) != copy(p[2:], ext.Bytes) {
			panic("unexpected extension length")
		}
		p = p[2+len(ext.Bytes):]
	}
	if len(p) != 0 {
		panic("header length changed")
	}
	return
}

var (
	_ net.Listener   = &Socket{}
	_ net.PacketConn = &Socket{}
)

const (
	csInvalid = iota
	csSynSent
	csConnected
	csDestroy
)

type st int

func (me st) String() string {
	switch me {
	case stData:
		return "stData"
	case stFin:
		return "stFin"
	case stState:
		return "stState"
	case stReset:
		return "stReset"
	case stSyn:
		return "stSyn"
	default:
		panic(fmt.Sprintf("%d", me))
	}
}

const (
	stData  st = 0
	stFin      = 1
	stState    = 2
	stReset    = 3
	stSyn      = 4

	// Used for validating packet headers.
	stMax = stSyn
)

// Conn is a uTP stream and implements net.Conn. It owned by a Socket, which
// handles dispatching packets to and from Conns.
type Conn struct {
	mu    sync.Mutex
	event sync.Cond

	recv_id, send_id uint16
	seq_nr, ack_nr   uint16
	lastAck          uint16
	lastTimeDiff     uint32
	peerWndSize      uint32

	readBuf []byte

	socket     *Socket
	remoteAddr net.Addr
	// The uTP timestamp.
	startTimestamp uint32
	// When the conn was allocated.
	created time.Time
	// Callback to unregister Conn from a parent Socket. Should be called when
	// no more packets will be handled.
	detach func()

	cs      int
	gotFin  bool
	sentFin bool
	err     error

	unackedSends []*send
	// Inbound payloads, the first is ack_nr+1.
	inbound   []recv
	packetsIn chan packet
	connDeadlines
	latencies        []time.Duration
	pendingSendState bool
	destroyed        chan struct{}
}

type send struct {
	acked       chan struct{} // Closed with Conn lock.
	payloadSize uint32
	started     time.Time
	// This send was skipped in a selective ack.
	resend   func()
	timedOut func()
	conn     *Conn

	mu          sync.Mutex
	acksSkipped int
	resendTimer *time.Timer
	numResends  int
}

func (s *send) Ack() (latency time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resendTimer.Stop()
	select {
	case <-s.acked:
		return
	default:
		close(s.acked)
	}
	latency = time.Since(s.started)
	return
}

type recv struct {
	seen bool
	data []byte
	Type st
}

var (
	_ net.Conn = &Conn{}
)

func (c *Conn) age() time.Duration {
	return time.Since(c.created)
}

func (c *Conn) timestamp() uint32 {
	return nowTimestamp() - c.startTimestamp
}

func (c *Conn) connected() bool {
	return c.cs == csConnected
}

// addr is used to create a listening UDP conn which becomes the underlying
// net.PacketConn for the Socket.
func NewSocket(network, addr string) (s *Socket, err error) {
	s = &Socket{
		backlog: make(map[syn]struct{}, backlog),
		reads:   make(chan read, 100),
		closing: make(chan struct{}),

		unusedReads: make(chan read, 100),
	}
	s.event.L = &s.mu
	s.pc, err = net.ListenPacket(network, addr)
	if err != nil {
		return
	}
	go s.reader()
	go s.dispatcher()
	return
}

func packetDebugString(h *header, payload []byte) string {
	return fmt.Sprintf("%s->%d: %q", h.Type, h.ConnID, payload)
}

func (s *Socket) reader() {
	defer close(s.reads)
	var b [maxRecvSize]byte
	for {
		if s.pc == nil {
			break
		}
		n, addr, err := s.pc.ReadFrom(b[:])
		if err != nil {
			select {
			case <-s.closing:
			default:
				s.ReadErr = err
			}
			return
		}
		var nilB []byte
		s.reads <- read{append(nilB, b[:n:n]...), addr}
	}
}

func (s *Socket) unusedRead(read read) {
	unusedReads.Add(1)
	select {
	case s.unusedReads <- read:
	default:
		// Drop the packet.
	}
}

func stringAddr(s string) net.Addr {
	addr, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		panic(err)
	}
	return addr
}

func (s *Socket) pushBacklog(syn syn) {
	if _, ok := s.backlog[syn]; ok {
		return
	}
	for k := range s.backlog {
		if len(s.backlog) < backlog {
			break
		}
		delete(s.backlog, k)
		// A syn is sent on the remote's recv_id, so this is where we can send
		// the reset.
		s.reset(stringAddr(k.addr), k.seq_nr, k.conn_id)
	}
	s.backlog[syn] = struct{}{}
	s.event.Broadcast()
}

func (s *Socket) dispatcher() {
	for {
		select {
		case read, ok := <-s.reads:
			if !ok {
				return
			}
			if len(read.data) < 20 {
				s.unusedRead(read)
				continue
			}
			s.dispatch(read)
		}
	}
}

func (s *Socket) dispatch(read read) {
	b := read.data
	addr := read.from
	var h header
	hEnd, err := h.Unmarshal(b)
	if logLevel >= 1 {
		log.Printf("recvd utp msg: %s", packetDebugString(&h, b[hEnd:]))
	}
	if err != nil || h.Type > stMax || h.Version != 1 {
		s.unusedRead(read)
		return
	}
	s.mu.RLock()
	c, ok := s.conns[connKey{resolvedAddrStr(addr.String()), func() (recvID uint16) {
		recvID = h.ConnID
		// If a SYN is resent, its connection ID field will be one lower
		// than we expect.
		if h.Type == stSyn {
			recvID++
		}
		return
	}()}]
	s.mu.RUnlock()
	if ok {
		if h.Type == stSyn {
			if h.ConnID == c.send_id-2 {
				// This is a SYN for connection that cannot exist locally. The
				// connection the remote wants to establish here with the proposed
				// recv_id, already has an existing connection that was dialled
				// *out* from this socket, which is why the send_id is 1 higher,
				// rather than 1 lower than the recv_id.
				log.Print("resetting conflicting syn")
				s.reset(addr, h.SeqNr, h.ConnID)
				return
			} else if h.ConnID != c.send_id {
				panic("bad assumption")
			}
		}
		c.deliver(h, b[hEnd:])
		return
	}
	if h.Type == stSyn {
		if logLevel >= 1 {
			log.Printf("adding SYN to backlog")
		}
		syn := syn{
			seq_nr:  h.SeqNr,
			conn_id: h.ConnID,
			addr:    addr.String(),
		}
		s.mu.Lock()
		s.pushBacklog(syn)
		s.mu.Unlock()
		return
	} else if h.Type != stReset {
		// This is an unexpected packet. We'll send a reset, but also pass
		// it on.
		// log.Print("resetting unexpected packet")
		// I don't think you can reset on the received packets ConnID if it isn't a SYN, as the send_id will differ in this case.
		s.reset(addr, h.SeqNr, h.ConnID)
		s.reset(addr, h.SeqNr, h.ConnID-1)
		s.reset(addr, h.SeqNr, h.ConnID+1)
	}
	s.unusedRead(read)
}

// Send a reset in response to a packet with the given header.
func (s *Socket) reset(addr net.Addr, ackNr, connId uint16) {
	go s.writeTo((&header{
		Type:    stReset,
		Version: 1,
		ConnID:  connId,
		AckNr:   ackNr,
	}).Marshal(), addr)
}

// Attempt to connect to a remote uTP listener, creating a Socket just for
// this connection.
func Dial(addr string) (net.Conn, error) {
	return DialTimeout(addr, 0)
}

// Same as Dial with a timeout parameter.
func DialTimeout(addr string, timeout time.Duration) (nc net.Conn, err error) {
	s, err := NewSocket("udp", ":0")
	if err != nil {
		return
	}
	return s.DialTimeout(addr, timeout)

}

// Return a recv_id that should be free. Handling the case where it isn't is
// deferred to a more appropriate function.
func (s *Socket) newConnID(remoteAddr resolvedAddrStr) (id uint16) {
	// Rather than use math.Rand, which requires generating all the IDs up
	// front and allocating a slice, we do it on the stack, generating the IDs
	// only as required. To do this, we use the fact that the array is
	// default-initialized. IDs that are 0, are actually their index in the
	// array. IDs that are non-zero, are +1 from their intended ID.
	var idsBack [0x10000]int
	ids := idsBack[:]
	for len(ids) != 0 {
		// Pick the next ID from the untried ids.
		i := rand.Intn(len(ids))
		id = uint16(ids[i])
		// If it's zero, then treat it as though the index i was the ID.
		// Otherwise the value we get is the ID+1.
		if id == 0 {
			id = uint16(i)
		} else {
			id--
		}
		// Check there's no connection using this ID for its recv_id...
		_, ok1 := s.conns[connKey{remoteAddr, id}]
		// and if we're connecting to our own Socket, that there isn't a Conn
		// already receiving on what will correspond to our send_id. Note that
		// we just assume that we could be connecting to our own Socket. This
		// will halve the available connection IDs to each distinct remote
		// address. Presumably that's ~0x8000, down from ~0x10000.
		_, ok2 := s.conns[connKey{remoteAddr, id + 1}]
		_, ok4 := s.conns[connKey{remoteAddr, id - 1}]
		if !ok1 && !ok2 && !ok4 {
			return
		}
		// The set of possible IDs is shrinking. The highest one will be lost, so
		// it's moved to the location of the one we just tried.
		ids[i] = len(ids) // Conveniently already +1.
		// And shrink.
		ids = ids[:len(ids)-1]
	}
	return
}

func (c *Conn) sendPendingState() {
	if !c.pendingSendState {
		return
	}
	c.sendState()
}

func (s *Socket) newConn(addr net.Addr) (c *Conn) {
	c = &Conn{
		socket:         s,
		remoteAddr:     addr,
		startTimestamp: nowTimestamp(),
		created:        time.Now(),
		packetsIn:      make(chan packet, 100),
		destroyed:      make(chan struct{}),
	}
	c.event.L = &c.mu
	c.mu.Lock()
	c.connDeadlines.read.setCallback(func() {
		c.mu.Lock()
		c.event.Broadcast()
		c.mu.Unlock()
	})
	c.connDeadlines.write.setCallback(func() {
		c.mu.Lock()
		c.event.Broadcast()
		c.mu.Unlock()
	})
	c.mu.Unlock()
	go c.deliveryProcessor()
	return
}

func (s *Socket) Dial(addr string) (net.Conn, error) {
	return s.DialTimeout(addr, 0)
}

func (s *Socket) DialTimeout(addr string, timeout time.Duration) (nc net.Conn, err error) {
	netAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return
	}

	s.mu.Lock()
	c := s.newConn(netAddr)
	c.recv_id = s.newConnID(resolvedAddrStr(netAddr.String()))
	c.send_id = c.recv_id + 1
	if logLevel >= 1 {
		log.Printf("dial registering addr: %s", netAddr.String())
	}
	if !s.registerConn(c.recv_id, resolvedAddrStr(netAddr.String()), c) {
		err = errors.New("couldn't register new connection")
		log.Println(c.recv_id, netAddr.String())
		for k, c := range s.conns {
			log.Println(k, c, c.age())
		}
		log.Printf("that's %d connections", len(s.conns))
	}
	s.mu.Unlock()
	if err != nil {
		return
	}

	connErr := make(chan error, 1)
	go func() {
		connErr <- c.connect()
	}()
	var timeoutCh <-chan time.Time
	if timeout != 0 {
		timeoutCh = time.After(timeout)
	}
	select {
	case err = <-connErr:
	case <-timeoutCh:
		c.Close()
		err = errTimeout
	}
	if err == nil {
		nc = c
	}
	return
}

func (c *Conn) wndSize() uint32 {
	if len(c.inbound) > maxUnackedInbound/2 {
		return 0
	}
	var buffered int
	for _, r := range c.inbound {
		buffered += len(r.data)
	}
	buffered += len(c.readBuf)
	if buffered >= recvWindow {
		return 0
	}
	return recvWindow - uint32(buffered)
}

func nowTimestamp() uint32 {
	return uint32(time.Now().UnixNano() / int64(time.Microsecond))
}

// Send the given payload with an up to date header.
func (c *Conn) send(_type st, connID uint16, payload []byte, seqNr uint16) (err error) {
	// Always selectively ack the first 64 packets. Don't bother with rest for
	// now.
	selAck := selectiveAckBitmask(make([]byte, 8))
	for i := 1; i < 65; i++ {
		if len(c.inbound) <= i {
			break
		}
		if c.inbound[i].seen {
			selAck.SetBit(i - 1)
		}
	}
	h := header{
		Type:          _type,
		Version:       1,
		ConnID:        connID,
		SeqNr:         seqNr,
		AckNr:         c.ack_nr,
		WndSize:       c.wndSize(),
		Timestamp:     c.timestamp(),
		TimestampDiff: c.lastTimeDiff,
		// Currently always send an 8 byte selective ack.
		Extensions: []extensionField{{
			Type:  extensionTypeSelectiveAck,
			Bytes: selAck,
		}},
	}
	p := h.Marshal()
	// Extension headers are currently fixed in size.
	if len(p) != maxHeaderSize {
		panic("header has unexpected size")
	}
	p = append(p, payload...)
	if logLevel >= 1 {
		log.Printf("writing utp msg to %s: %s", c.remoteAddr, packetDebugString(&h, payload))
	}
	n1, err := c.socket.writeTo(p, c.remoteAddr)
	if err != nil {
		return
	}
	if n1 != len(p) {
		panic(n1)
	}
	c.unpendSendState()
	return
}

func (me *Conn) unpendSendState() {
	me.pendingSendState = false
}

func (c *Conn) pendSendState() {
	c.pendingSendState = true
}

func (me *Socket) writeTo(b []byte, addr net.Addr) (n int, err error) {
	mu.RLock()
	apdc := artificialPacketDropChance
	mu.RUnlock()
	if apdc != 0 {
		if rand.Float64() < apdc {
			n = len(b)
			return
		}
	}
	n, err = me.pc.WriteTo(b, addr)
	return
}

func (s *send) timeoutResend() {
	select {
	case <-s.acked:
		return
	default:
	}
	if time.Since(s.started) >= sendTimeout {
		s.timedOut()
		return
	}
	s.conn.mu.Lock()
	rt := s.conn.resendTimeout()
	s.conn.mu.Unlock()
	go s.resend()
	s.mu.Lock()
	s.numResends++
	s.resendTimer.Reset(rt)
	s.mu.Unlock()
}

func (me *Conn) writeSyn() (err error) {
	if me.cs != csInvalid {
		panic(me.cs)
	}
	_, err = me.write(stSyn, me.recv_id, nil, me.seq_nr)
	return
}

func (c *Conn) write(_type st, connID uint16, payload []byte, seqNr uint16) (n int, err error) {
	switch _type {
	case stSyn, stFin, stData:
	default:
		panic(_type)
	}
	switch c.cs {
	case csConnected, csSynSent, csInvalid:
	default:
		panic(c.cs)
	}
	if c.sentFin {
		panic(c)
	}
	if len(payload) > maxPayloadSize {
		payload = payload[:maxPayloadSize]
	}
	err = c.send(_type, connID, payload, seqNr)
	if err != nil {
		return
	}
	n = len(payload)
	// Copy payload so caller to write can continue to use the buffer.
	if payload != nil {
		payload = append(sendBufferPool.Get().([]byte)[:0:minMTU], payload...)
	}
	send := &send{
		acked:       make(chan struct{}),
		payloadSize: uint32(len(payload)),
		started:     time.Now(),
		resend: func() {
			c.mu.Lock()
			err := c.send(_type, connID, payload, seqNr)
			if err != nil {
				log.Printf("error resending packet: %s", err)
			}
			c.mu.Unlock()
		},
		timedOut: func() {
			c.mu.Lock()
			c.destroy(errAckTimeout)
			c.mu.Unlock()
		},
		conn: c,
	}
	send.mu.Lock()
	send.resendTimer = time.AfterFunc(c.resendTimeout(), send.timeoutResend)
	send.mu.Unlock()
	c.unackedSends = append(c.unackedSends, send)
	c.seq_nr++
	return
}

func (c *Conn) latency() (ret time.Duration) {
	if len(c.latencies) == 0 {
		// Sort of the p95 of latencies?
		return 200 * time.Millisecond
	}
	for _, l := range c.latencies {
		ret += l
	}
	ret = (ret + time.Duration(len(c.latencies)) - 1) / time.Duration(len(c.latencies))
	return
}

func (c *Conn) numUnackedSends() (num int) {
	for _, s := range c.unackedSends {
		select {
		case <-s.acked:
		default:
			num++
		}
	}
	return
}

func (c *Conn) cur_window() (window uint32) {
	for _, s := range c.unackedSends {
		select {
		case <-s.acked:
		default:
			window += s.payloadSize
		}
	}
	return
}

func (c *Conn) sendState() {
	c.send(stState, c.send_id, nil, c.seq_nr)
	sentStatePackets.Add(1)
}

func seqLess(a, b uint16) bool {
	if b < 0x8000 {
		return a < b || a >= b-0x8000
	} else {
		return a < b && a >= b-0x8000
	}
}

// Ack our send with the given sequence number.
func (c *Conn) ack(nr uint16) {
	if !seqLess(c.lastAck, nr) {
		// Already acked.
		return
	}
	i := nr - c.lastAck - 1
	if int(i) >= len(c.unackedSends) {
		log.Printf("got ack ahead of syn (%x > %x)", nr, c.seq_nr-1)
		return
	}
	latency := c.unackedSends[i].Ack()
	if latency != 0 {
		c.latencies = append(c.latencies, latency)
		if len(c.latencies) > 10 {
			c.latencies = c.latencies[len(c.latencies)-10:]
		}
	}
	for {
		if len(c.unackedSends) == 0 {
			break
		}
		select {
		case <-c.unackedSends[0].acked:
		default:
			// Can't trim unacked sends any further.
			return
		}
		// Trim the front of the unacked sends.
		c.unackedSends = c.unackedSends[1:]
		c.lastAck++
	}
	c.event.Broadcast()
}

func (c *Conn) ackTo(nr uint16) {
	if !seqLess(nr, c.seq_nr) {
		return
	}
	for seqLess(c.lastAck, nr) {
		c.ack(c.lastAck + 1)
	}
}

type selectiveAckBitmask []byte

func (me selectiveAckBitmask) NumBits() int {
	return len(me) * 8
}

func (me selectiveAckBitmask) SetBit(index int) {
	me[index/8] |= 1 << uint(index%8)
}

func (me selectiveAckBitmask) BitIsSet(index int) bool {
	return me[index/8]>>uint(index%8)&1 == 1
}

// Return the send state for the sequence number. Returns nil if there's no
// outstanding send for that sequence number.
func (c *Conn) seqSend(seqNr uint16) *send {
	if !seqLess(c.lastAck, seqNr) {
		// Presumably already acked.
		return nil
	}
	i := int(seqNr - c.lastAck - 1)
	if i >= len(c.unackedSends) {
		// No such send.
		return nil
	}
	return c.unackedSends[i]
}

func (c *Conn) resendTimeout() time.Duration {
	l := c.latency()
	if l < 10*time.Millisecond {
		l = 10 * time.Millisecond
	}
	ret := jitter.Duration(3*l, l)
	// log.Print(ret)
	return ret
}

func (c *Conn) ackSkipped(seqNr uint16) {
	send := c.seqSend(seqNr)
	if send == nil {
		return
	}
	send.mu.Lock()
	defer send.mu.Unlock()
	send.acksSkipped++
	switch send.acksSkipped {
	case 3, 60:
		ackSkippedResends.Add(1)
		go send.resend()
		send.resendTimer.Reset(c.resendTimeout())
	default:
	}
}

type packet struct {
	h       header
	payload []byte
}

func (c *Conn) deliver(h header, payload []byte) {
	c.packetsIn <- packet{h, payload}
}

func (c *Conn) deliveryProcessor() {
	for {
		select {
		case p := <-c.packetsIn:
			c.processDelivery(p.h, p.payload)
			timeout := time.After(500 * time.Microsecond)
		batched:
			for {
				select {
				case p := <-c.packetsIn:
					c.processDelivery(p.h, p.payload)
				case <-timeout:
					break batched
				}
			}
			c.mu.Lock()
			c.sendPendingState()
			c.mu.Unlock()
		case <-c.destroyed:
			return
		}
	}
}

func (c *Conn) processDelivery(h header, payload []byte) {
	deliveriesProcessed.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()
	defer c.event.Broadcast()
	c.assertHeader(h)
	c.peerWndSize = h.WndSize
	c.applyAcks(h)
	if h.Timestamp == 0 {
		c.lastTimeDiff = 0
	} else {
		c.lastTimeDiff = c.timestamp() - h.Timestamp
	}

	// We want this connection destroyed, and our peer has acked everything.
	if c.sentFin && len(c.unackedSends) == 0 {
		// log.Print("gracefully completed")
		c.destroy(nil)
		return
	}
	if h.Type == stReset {
		c.destroy(errors.New("peer reset"))
		return
	}
	if c.cs == csSynSent {
		if h.Type != stState {
			return
		}
		c.changeState(csConnected)
		c.ack_nr = h.SeqNr - 1
		return
	}
	if h.Type == stState {
		return
	}
	c.pendSendState()
	if !seqLess(c.ack_nr, h.SeqNr) {
		// Already received this packet.
		return
	}
	inboundIndex := int(h.SeqNr - c.ack_nr - 1)
	if inboundIndex < len(c.inbound) && c.inbound[inboundIndex].seen {
		// Already received this packet.
		return
	}
	// Derived from running in production:
	// grep -oP '(?<=packet out of order, index=)\d+' log | sort -n | uniq -c
	// 64 should correspond to 8 bytes of selective ack.
	if inboundIndex >= maxUnackedInbound {
		// Discard packet too far ahead.
		if missinggo.CryHeard() {
			// I can't tell if this occurs due to bad peers, or something
			// missing in the implementation.
			log.Printf("received packet from %s %d ahead of next seqnr (%x > %x)", c.remoteAddr, inboundIndex, h.SeqNr, c.ack_nr+1)
		}
		return
	}
	// Extend inbound so the new packet has a place.
	for inboundIndex >= len(c.inbound) {
		c.inbound = append(c.inbound, recv{})
	}
	c.inbound[inboundIndex] = recv{true, payload, h.Type}
	c.processInbound()
}

func (c *Conn) applyAcks(h header) {
	c.ackTo(h.AckNr)
	for _, ext := range h.Extensions {
		switch ext.Type {
		case extensionTypeSelectiveAck:
			c.ackSkipped(h.AckNr + 1)
			bitmask := selectiveAckBitmask(ext.Bytes)
			for i := 0; i < bitmask.NumBits(); i++ {
				if bitmask.BitIsSet(i) {
					nr := h.AckNr + 2 + uint16(i)
					// log.Printf("selectively acked %d", nr)
					c.ack(nr)
				} else {
					c.ackSkipped(h.AckNr + 2 + uint16(i))
				}
			}
		}
	}
}

func (c *Conn) assertHeader(h header) {
	if h.Type == stSyn {
		if h.ConnID != c.send_id {
			panic(fmt.Sprintf("%d != %d", h.ConnID, c.send_id))
		}
	} else {
		if h.ConnID != c.recv_id {
			panic("erroneous delivery")
		}
	}
}

func (c *Conn) processInbound() {
	// Consume consecutive next packets.
	for !c.gotFin && len(c.inbound) > 0 && c.inbound[0].seen {
		c.ack_nr++
		p := c.inbound[0]
		c.inbound = c.inbound[1:]
		c.readBuf = append(c.readBuf, p.data...)
		if p.Type == stFin {
			c.gotFin = true
		}
	}
}

func (c *Conn) waitAck(seq uint16) {
	send := c.seqSend(seq)
	if send == nil {
		return
	}
	c.mu.Unlock()
	defer c.mu.Lock()
	<-send.acked
	return
}

func (c *Conn) changeState(cs int) {
	// log.Println(c, "goes", c.cs, "->", cs)
	c.cs = cs
}

func (c *Conn) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq_nr = 1
	err := c.writeSyn()
	if err != nil {
		return err
	}
	c.changeState(csSynSent)
	if logLevel >= 2 {
		log.Printf("sent syn")
	}
	// c.seq_nr++
	c.waitAck(1)
	if c.err != nil {
		err = c.err
	}
	c.event.Broadcast()
	return err
}

// Returns true if the connection was newly registered, false otherwise.
func (s *Socket) registerConn(recvID uint16, remoteAddr resolvedAddrStr, c *Conn) bool {
	if s.conns == nil {
		s.conns = make(map[connKey]*Conn)
	}
	key := connKey{remoteAddr, recvID}
	if _, ok := s.conns[key]; ok {
		return false
	}
	s.conns[key] = c
	c.detach = func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		defer s.event.Broadcast()
		if s.conns[key] != c {
			panic("conn changed")
		}
		// log.Println("detached", key)
		delete(s.conns, key)
		if len(s.conns) == 0 {
			s.pc.Close()
		}
	}
	return true
}

func (s *Socket) nextSyn() (syn syn, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		for k := range s.backlog {
			syn = k
			delete(s.backlog, k)
			ok = true
			return
		}
		select {
		case <-s.closing:
			return
		default:
		}
		s.event.Wait()
	}
}

// Accept and return a new uTP connection.
func (s *Socket) Accept() (c net.Conn, err error) {
	for {
		syn, ok := s.nextSyn()
		if !ok {
			err = errClosed
			return
		}
		s.mu.Lock()
		_c := s.newConn(stringAddr(syn.addr))
		_c.send_id = syn.conn_id
		_c.recv_id = _c.send_id + 1
		_c.seq_nr = uint16(rand.Int())
		_c.lastAck = _c.seq_nr - 1
		_c.ack_nr = syn.seq_nr
		_c.cs = csConnected
		if !s.registerConn(_c.recv_id, resolvedAddrStr(syn.addr), _c) {
			// SYN that triggered this accept duplicates existing connection.
			// Ack again in case the SYN was a resend.
			_c = s.conns[connKey{resolvedAddrStr(syn.addr), _c.recv_id}]
			if _c.send_id != syn.conn_id {
				panic(":|")
			}
			_c.sendState()
			s.mu.Unlock()
			continue
		}
		_c.sendState()
		// _c.seq_nr++
		c = _c
		s.mu.Unlock()
		return
	}
}

// The address we're listening on for new uTP connections.
func (s *Socket) Addr() net.Addr {
	return s.pc.LocalAddr()
}

// Marks the Socket for close. Currently this just axes the underlying OS
// socket.
func (s *Socket) Close() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case <-s.closing:
		return
	default:
	}
	s.event.Broadcast()
	close(s.closing)
	if len(s.conns) == 0 {
		err = s.pc.Close()
	}
	return
}

func (s *Socket) LocalAddr() net.Addr {
	return s.pc.LocalAddr()
}

func (s *Socket) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	read, ok := <-s.unusedReads
	if !ok {
		err = io.EOF
	}
	n = copy(p, read.data)
	addr = read.from
	return
}

func (s *Socket) WriteTo(b []byte, addr net.Addr) (int, error) {
	return s.pc.WriteTo(b, addr)
}

func (c *Conn) writeFin() (err error) {
	if c.sentFin {
		return
	}
	_, err = c.write(stFin, c.send_id, nil, c.seq_nr)
	if err != nil {
		return
	}
	c.sentFin = true
	c.event.Broadcast()
	return
}

func (c *Conn) destroy(reason error) {
	if c.err != nil && reason != nil {
		log.Printf("duplicate destroy call: %s", reason)
	}
	if c.cs == csDestroy {
		return
	}
	close(c.destroyed)
	c.writeFin()
	c.changeState(csDestroy)
	c.err = reason
	c.event.Broadcast()
	c.detach()
	for _, s := range c.unackedSends {
		s.Ack()
	}
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writeFin()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.socket.Addr()
}

func (c *Conn) Read(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		if len(c.readBuf) != 0 {
			break
		}
		if c.cs == csDestroy || c.gotFin || c.sentFin {
			err = c.err
			if err == nil {
				err = io.EOF
			}
			return
		}
		if c.connDeadlines.read.deadlineExceeded() {
			err = errTimeout
			return
		}
		if logLevel >= 2 {
			log.Printf("nothing to read, state=%d", c.cs)
		}
		c.event.Wait()
	}
	n = copy(b, c.readBuf)
	c.readBuf = c.readBuf[n:]

	return
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *Conn) String() string {
	return fmt.Sprintf("<UTPConn %s-%s (%d)>", c.LocalAddr(), c.RemoteAddr(), c.recv_id)
}

func (c *Conn) Write(p []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(p) != 0 {
		for {
			if c.sentFin {
				err = io.ErrClosedPipe
				return
			}
			// If peerWndSize is 0, we still want to send something, so don't
			// block until we exceed it.
			if c.cur_window() <= c.peerWndSize && len(c.unackedSends) < 64 && c.cs == csConnected {
				break
			}
			if c.connDeadlines.write.deadlineExceeded() {
				err = errTimeout
				return
			}
			c.event.Wait()
		}
		var n1 int
		n1, err = c.write(stData, c.send_id, p, c.seq_nr)
		if err != nil {
			return
		}
		// c.seq_nr++
		n += n1
		p = p[n1:]
	}
	return
}
