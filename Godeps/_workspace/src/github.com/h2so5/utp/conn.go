package utp

import (
	"math"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Conn is an implementation of the Conn interface for UTP network
// connections.
type Conn struct {
	conn                        *baseConn
	raddr                       net.Addr
	rid, sid, seq, ack, lastAck uint16
	rtt, rttVar, minRtt, rto    int64
	dupAck                      int
	diff, maxWindow             uint32

	state  int
	closed int32

	recvbuf *packetBuffer
	sendbuf *packetBuffer

	readbuf  *byteRingBuffer
	writebuf *rateLimitedBuffer

	baseDelay baseDelayBuffer

	writech chan []byte
	ackch   chan int
	synch   chan int

	rdeadline     time.Time
	wdeadline     time.Time
	deadlineMutex sync.RWMutex

	recv chan *packet

	closing   bool
	closingch chan int

	keepalivech chan time.Duration

	connch chan int

	closech      chan int
	closechMutex sync.Mutex

	stat statistics
}

type statistics struct {
	sentPackets            int
	resentPackets          int
	receivedPackets        int
	receivedDuplicatedACKs int
	packetTimedOuts        int
	sentSelectiveACKs      int
	receivedSelectiveACKs  int
	rtoSum                 int64
	rtoCount               int
}

func newConn() *Conn {
	wch := make(chan []byte)
	c := &Conn{
		minRtt:    math.MaxInt64,
		maxWindow: mss,
		rto:       int64(60),

		recv:   make(chan *packet),
		connch: make(chan int),

		recvbuf: newPacketBuffer(0, 0),

		readbuf:  newByteRingBuffer(readBufferSize),
		writebuf: newRateLimitedBuffer(wch, mss),

		writech: wch,
		ackch:   make(chan int),
		synch:   make(chan int),

		closingch:   make(chan int),
		keepalivech: make(chan time.Duration),
		closech:     make(chan int),
	}
	return c
}

func (c *Conn) ok() bool { return c != nil && c.conn != nil }

// Close closes the connection.
func (c *Conn) Close() error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if !c.isOpen() {
		return nil
	}
	select {
	case <-c.closingch:
	default:
		close(c.closingch)
	}
	select {
	case <-c.connch:
	default:
		return nil
	}
	<-c.closech
	return nil
}

// LocalAddr returns the local network address.
func (c *Conn) LocalAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (c *Conn) RemoteAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return c.raddr
}

// Read implements the Conn Read method.
func (c *Conn) Read(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	if !c.isOpen() {
		return 0, &net.OpError{
			Op:   "read",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  errClosing,
		}
	}
	s := c.readbuf.space()
	c.deadlineMutex.RLock()
	d := timeToDeadline(c.rdeadline)
	c.deadlineMutex.RUnlock()
	l, err := c.readbuf.ReadTimeout(b, d)
	if s < mss && c.readbuf.space() > 0 {
		select {
		case c.ackch <- 0:
		default:
		}
	}
	return l, err
}

func timeToDeadline(deadline time.Time) (d time.Duration) {
	if deadline.IsZero() {
		return
	}
	d = deadline.Sub(time.Now())
	if d < 0 {
		d = 0
	}
	return
}

// Write implements the Conn Write method.
func (c *Conn) Write(b []byte) (int, error) {
	if !c.ok() {
		return 0, syscall.EINVAL
	}
	if !c.isOpen() {
		return 0, &net.OpError{
			Op:   "write",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  errClosing,
		}
	}
	c.deadlineMutex.RLock()
	d := timeToDeadline(c.wdeadline)
	c.deadlineMutex.RUnlock()
	return c.writebuf.WriteTimeout(b, d)
}

// SetDeadline implements the Conn SetDeadline method.
func (c *Conn) SetDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

// SetReadDeadline implements the Conn SetReadDeadline method.
func (c *Conn) SetReadDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	c.deadlineMutex.Lock()
	defer c.deadlineMutex.Unlock()
	c.rdeadline = t
	return nil
}

// SetWriteDeadline implements the Conn SetWriteDeadline method.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	c.deadlineMutex.Lock()
	defer c.deadlineMutex.Unlock()
	c.wdeadline = t
	return nil
}

// SetKeepAlive sets the keepalive interval associated with the connection.
func (c *Conn) SetKeepAlive(d time.Duration) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if !c.isOpen() {
		return errClosing
	}
	c.keepalivech <- d
	return nil
}

func (c *Conn) loop() {
	defer c.conn.Unregister(int32(c.rid))

	var resendSeq uint16
	var resendCont int
	var keepalive <-chan time.Time

	resend := time.NewTimer(0)
	resend.Stop()
	defer resend.Stop()

	for {
		resend.Stop()
		f := c.sendbuf.front()
		if f != nil {
			resend.Reset(time.Duration(c.rto) * time.Millisecond)
		}
		select {
		case <-c.ackch:
			c.sendACK()
		case <-c.synch:
			c.sendSYN()
		case p := <-c.recv:
			c.stat.receivedPackets++
			c.processPacket(p)
		case b := <-c.writech:
			c.sendDATA(b)
		case <-c.closingch:
			c.enterClosing()
		case <-resend.C:
			if f != nil {
				if resendSeq == f.header.seq {
					resendCont++
				} else {
					resendCont = 0
					resendSeq = f.header.seq
				}
				c.stat.packetTimedOuts++
				if resendCont > maxRetry {
					c.sendRST()
					c.close()
				} else {
					c.maxWindow /= 2
					if c.maxWindow < mtu {
						c.maxWindow = mtu
					}
					for _, p := range c.sendbuf.sequence() {
						c.resend(p)
					}
				}
			}
		case <-c.closech:
			c.readbuf.Close()
			c.state = stateClosed
			atomic.StoreInt32(&c.closed, 1)
			return
		case d := <-c.keepalivech:
			if d <= 0 {
				keepalive = nil
			} else {
				keepalive = time.Tick(d)
			}
		case <-keepalive:
			ulog.Printf(2, "Conn(%v): Send keepalive", c.LocalAddr())
			c.sendACK()
		}
		if c.closing {
			c.tryFIN()
			if c.state == stateSynSent || c.state == stateFinSent || (c.recvbuf.empty() && c.sendbuf.empty()) {
				c.close()
			}
		}
	}
}

func (c *Conn) tryFIN() {
	if c.state != stateFinSent {
		if c.sendFIN() == nil {
			c.writebuf.Close()
			c.state = stateFinSent
		}
	}
}

func (c *Conn) enterClosing() {
	if !c.closing {
		c.closing = true
	}
}

func (c *Conn) close() {
	c.closechMutex.Lock()
	defer c.closechMutex.Unlock()
	select {
	case <-c.closech:
	default:
		close(c.closech)
	}
	ulog.Printf(1, "Conn(%v): closed", c.LocalAddr())
	ulog.Printf(1, "Conn(%v): * SentPackets: %d", c.LocalAddr(), c.stat.sentPackets)
	ulog.Printf(1, "Conn(%v): * ResentPackets: %d", c.LocalAddr(), c.stat.resentPackets)
	ulog.Printf(1, "Conn(%v): * ReceivedPackets: %d", c.LocalAddr(), c.stat.receivedPackets)
	ulog.Printf(1, "Conn(%v): * ReceivedDuplicatedACKs: %d", c.LocalAddr(), c.stat.receivedDuplicatedACKs)
	ulog.Printf(1, "Conn(%v): * PacketTimedOuts: %d", c.LocalAddr(), c.stat.packetTimedOuts)
	ulog.Printf(1, "Conn(%v): * SentSelectiveACKs: %d", c.LocalAddr(), c.stat.sentSelectiveACKs)
	ulog.Printf(1, "Conn(%v): * ReceivedSelectiveACKs: %d", c.LocalAddr(), c.stat.receivedSelectiveACKs)
	if c.stat.rtoCount > 0 {
		ulog.Printf(1, "Conn(%v): * AverageRTO: %d", c.LocalAddr(), c.stat.rtoSum/int64(c.stat.rtoCount))
	}
}

func (c *Conn) isOpen() bool {
	return atomic.LoadInt32(&c.closed) == 0
}

func currentMicrosecond() uint32 {
	return uint32(time.Now().Nanosecond() / 1000)
}

func (c *Conn) processPacket(p *packet) {
	if p.header.t == 0 {
		c.diff = 0
	} else {
		t := currentMicrosecond()
		if t > p.header.t {
			c.diff = t - p.header.t
			if c.minRtt > int64(c.diff) {
				c.minRtt = int64(c.diff)
			}
		}
	}

	c.baseDelay.Push(c.diff)

	switch p.header.typ {
	case stState:
		f := c.sendbuf.front()
		if f != nil && p.header.ack == f.header.seq {
			for _, e := range p.ext {
				if e.typ == extSelectiveAck {
					ulog.Printf(3, "Conn(%v): Receive Selective ACK", c.LocalAddr())
					c.stat.receivedSelectiveACKs++
					c.sendbuf.processSelectiveACK(e.payload)
				}
			}
		}

		s := c.sendbuf.fetch(p.header.ack)
		if s != nil {
			current := currentMicrosecond()
			if current > s.header.t {
				e := int64(current-s.header.t) / 1000
				if c.rtt == 0 {
					c.rtt = e
					c.rttVar = e / 2
				} else {
					d := c.rtt - e
					if d < 0 {
						d = -d
					}
					c.rttVar += (d - c.rttVar) / 4
					c.rtt = c.rtt - c.rtt/8 + e/8
				}
				c.rto = c.rtt + c.rttVar*4
				if c.rto < 60 {
					c.rto = 60
				} else if c.rto > 1000 {
					c.rto = 1000
				}
				c.stat.rtoSum += c.rto
				c.stat.rtoCount++
			}

			ourDelay := float64(c.diff - c.baseDelay.Min())
			if ourDelay != 0.0 {
				offTarget := 100000.0 - ourDelay
				windowFactor := float64(mtu) / float64(c.maxWindow)
				delayFactor := offTarget / 100000.0
				gain := 3000.0 * delayFactor * windowFactor
				c.maxWindow = uint32(int(c.maxWindow) + int(gain))
				if c.maxWindow < mtu {
					c.maxWindow = mtu
				}
				ulog.Printf(4, "Conn(%v): Update maxWindow: %d", c.LocalAddr(), c.maxWindow)
			}
		}

		c.sendbuf.compact()

		if c.lastAck == p.header.ack {
			c.dupAck++
			if c.dupAck >= 2 {
				c.stat.receivedDuplicatedACKs++
				ulog.Printf(3, "Conn(%v): Receive 3 duplicated acks: %d", c.LocalAddr(), p.header.ack)
				p := c.sendbuf.front()
				if p != nil {
					c.maxWindow /= 2
					if c.maxWindow < mtu {
						c.maxWindow = mtu
					}
					ulog.Printf(4, "Conn(%v): Update maxWindow: %d", c.LocalAddr(), c.maxWindow)
					c.resend(p)
				}
				c.dupAck = 0
			}
		} else {
			c.dupAck = 0
		}

		c.lastAck = p.header.ack
		if p.header.ack == c.seq-1 {
			wnd := p.header.wnd
			if wnd > c.maxWindow {
				wnd = c.maxWindow
			}
			c.writebuf.Reset(wnd)
		}

		if c.state == stateSynSent {
			c.recvbuf = newPacketBuffer(windowSize, int(p.header.seq))
			c.state = stateConnected
			close(c.connch)
		}

	case stReset:
		c.sendRST()
		c.close()

	default:
		c.recvbuf.push(p)
		for _, s := range c.recvbuf.fetchSequence() {
			c.ack = s.header.seq
			if s.header.typ == stData {
				c.readbuf.Write(s.payload)
			} else if s.header.typ == stFin {
				c.enterClosing()
			}
		}
		c.sendACK()
	}
}

func (c *Conn) sendACK() {
	ack := c.makePacket(stState, nil, c.raddr)
	selack := c.sendbuf.generateSelectiveACK()
	if selack != nil {
		c.stat.sentSelectiveACKs++
		ack.ext = []extension{
			extension{
				typ:     extSelectiveAck,
				payload: selack,
			},
		}
	}
	c.stat.sentPackets++
	c.conn.Send(ack)
}

func (c *Conn) sendSYN() {
	syn := c.makePacket(stSyn, nil, c.raddr)
	err := c.sendbuf.push(syn)
	if err != nil {
		ulog.Printf(2, "Conn(%v): buffer error: %v", c.LocalAddr(), err)
		return
	}
	c.stat.sentPackets++
	c.conn.Send(syn)
}

func (c *Conn) sendFIN() error {
	fin := c.makePacket(stFin, nil, c.raddr)
	err := c.sendbuf.push(fin)
	if err != nil {
		ulog.Printf(2, "Conn(%v): buffer error: %v", c.LocalAddr(), err)
		return err
	}
	c.stat.sentPackets++
	c.conn.Send(fin)
	return nil
}

func (c *Conn) sendRST() {
	rst := c.makePacket(stReset, nil, c.raddr)
	c.stat.sentPackets++
	c.conn.Send(rst)
}

func (c *Conn) sendDATA(b []byte) {
	for i := 0; i <= len(b)/mss; i++ {
		l := len(b) - i*mss
		if l > mss {
			l = mss
		}
		data := c.makePacket(stData, b[i*mss:i*mss+l], c.raddr)
		c.sendbuf.push(data)
		c.stat.sentPackets++
		c.conn.Send(data)
	}
}

func (c *Conn) resend(p *packet) {
	c.stat.resentPackets++
	c.conn.Send(p)
	ulog.Printf(3, "Conn(%v): RESEND: %s", c.LocalAddr(), p.String())
}

func (c *Conn) makePacket(typ int, payload []byte, dst net.Addr) *packet {
	wnd := windowSize * mtu
	if c.recvbuf != nil {
		wnd = c.recvbuf.space() * mtu
	}
	s := c.readbuf.space()
	if wnd > s {
		wnd = s
	}
	id := c.sid
	if typ == stSyn {
		id = c.rid
	}
	p := &packet{}
	p.header.typ = typ
	p.header.ver = version
	p.header.id = id
	p.header.t = currentMicrosecond()
	p.header.diff = c.diff
	p.header.wnd = uint32(wnd)
	p.header.seq = c.seq
	p.header.ack = c.ack
	p.addr = dst
	if typ != stState && typ != stFin {
		c.seq++
	}
	p.payload = payload
	return p
}
