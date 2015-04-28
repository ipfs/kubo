package utp

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var baseConnMap = make(map[string]*baseConn)
var baseConnMutex sync.Mutex

type packetHandler struct {
	send   chan<- *packet
	closed chan int
}

type baseConn struct {
	addr             string
	conn             net.PacketConn
	synPackets       *packetRingBuffer
	outOfBandPackets *packetRingBuffer

	handlers     map[uint16]*packetHandler
	handlerMutex sync.RWMutex
	ref          int32
	refMutex     sync.RWMutex

	rdeadline time.Time
	wdeadline time.Time

	softClosed int32
	closed     int32
}

func newBaseConn(n string, addr *Addr) (*baseConn, error) {
	udpnet, err := utp2udp(n)
	if err != nil {
		return nil, err
	}
	var s string
	if addr != nil {
		s = addr.String()
	} else {
		s = ":0"
	}
	conn, err := net.ListenPacket(udpnet, s)
	if err != nil {
		return nil, err
	}
	c := &baseConn{
		conn:             conn,
		synPackets:       newPacketRingBuffer(packetBufferSize),
		outOfBandPackets: newPacketRingBuffer(packetBufferSize),
		handlers:         make(map[uint16]*packetHandler),
	}
	c.Register(-1, nil)
	go c.recvLoop()
	return c, nil
}

func getSharedBaseConn(n string, addr *Addr) (*baseConn, error) {
	baseConnMutex.Lock()
	defer baseConnMutex.Unlock()
	var s string
	if addr != nil {
		s = addr.String()
	} else {
		s = ":0"
	}
	if c, ok := baseConnMap[s]; ok {
		return c, nil
	}
	c, err := newBaseConn(n, addr)
	if err != nil {
		return nil, err
	}
	c.addr = s
	baseConnMap[s] = c
	go c.recvLoop()
	return c, nil
}

func (c *baseConn) ok() bool { return c != nil && c.conn != nil }

func (c *baseConn) LocalAddr() net.Addr {
	if !c.ok() {
		return nil
	}
	return &Addr{Addr: c.conn.LocalAddr()}
}

func (c *baseConn) ReadFrom(b []byte) (n int, addr net.Addr, err error) {
	if !c.ok() {
		return 0, nil, syscall.EINVAL
	}
	if !c.isOpen() {
		return 0, nil, &net.OpError{
			Op:   "read",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  errClosing,
		}
	}
	var d time.Duration
	if !c.rdeadline.IsZero() {
		d = c.rdeadline.Sub(time.Now())
		if d < 0 {
			d = 0
		}
	}
	p, err := c.outOfBandPackets.popOne(d)
	if err != nil {
		return 0, nil, &net.OpError{
			Op:   "read",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  err,
		}
	}
	return copy(b, p.payload), p.addr, nil
}

func (c *baseConn) WriteTo(b []byte, addr net.Addr) (n int, err error) {
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
	return c.conn.WriteTo(b, addr)
}

func (c *baseConn) Close() error {
	if !c.ok() {
		return syscall.EINVAL
	}
	if c.isOpen() && atomic.CompareAndSwapInt32(&c.softClosed, 0, 1) {
		c.Unregister(-1)
	} else {
		return &net.OpError{
			Op:   "close",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  errClosing,
		}
	}
	return nil
}

func (c *baseConn) SetDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *baseConn) SetReadDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	c.rdeadline = t
	return nil
}

func (c *baseConn) SetWriteDeadline(t time.Time) error {
	if !c.ok() {
		return syscall.EINVAL
	}
	c.wdeadline = t
	return nil
}

func (c *baseConn) recvLoop() {
	var buf [maxUdpPayload]byte
	for {
		l, addr, err := c.conn.ReadFrom(buf[:])
		if err != nil {
			ulog.Printf(3, "baseConn(%v): %v", c.LocalAddr(), err)
			return
		}
		p, err := c.decodePacket(buf[:l])
		if err != nil {
			ulog.Printf(3, "baseConn(%v): RECV out-of-band packet (len: %d) from %v", c.LocalAddr(), l, addr)
			c.outOfBandPackets.push(&packet{payload: append([]byte{}, buf[:l]...), addr: addr})
		} else {
			p.addr = addr
			ulog.Printf(3, "baseConn(%v): RECV: %v from %v", c.LocalAddr(), p, addr)
			if p.header.typ == stSyn {
				// ignore duplicated syns
				if !c.exists(p.header.id + 1) {
					c.synPackets.push(p)
				}
			} else {
				c.processPacket(p)
			}
		}
	}
}

func (c *baseConn) decodePacket(b []byte) (*packet, error) {
	var p packet
	err := p.UnmarshalBinary(b)
	if err != nil {
		return nil, err
	}
	if p.header.ver != version {
		return nil, errors.New("unsupported utp version")
	}
	return &p, nil
}

func (c *baseConn) exists(id uint16) bool {
	c.handlerMutex.RLock()
	defer c.handlerMutex.RUnlock()
	return c.handlers[id] != nil
}

func (c *baseConn) processPacket(p *packet) {
	c.handlerMutex.RLock()
	h, ok := c.handlers[p.header.id]
	c.handlerMutex.RUnlock()
	if ok {
		select {
		case <-h.closed:
		case h.send <- p:
		}
	}
}

func (c *baseConn) Register(id int32, f chan<- *packet) {
	if id < 0 {
		c.refMutex.Lock()
		c.ref++
		c.refMutex.Unlock()
	} else {
		if f == nil {
			panic("nil handler not allowed")
		}
		c.handlerMutex.Lock()
		_, ok := c.handlers[uint16(id)]
		c.handlerMutex.Unlock()
		if !ok {
			c.refMutex.Lock()
			c.ref++
			c.refMutex.Unlock()
			c.handlerMutex.Lock()
			c.handlers[uint16(id)] = &packetHandler{
				send:   f,
				closed: make(chan int),
			}
			c.handlerMutex.Unlock()
			ulog.Printf(2, "baseConn(%v): register #%d (ref: %d)", c.LocalAddr(), id, c.ref)
		}
	}
}

func (c *baseConn) Unregister(id int32) {
	if id < 0 {
		c.refMutex.Lock()
		c.ref--
		c.refMutex.Unlock()
	} else {
		c.handlerMutex.Lock()
		f, ok := c.handlers[uint16(id)]
		c.handlerMutex.Unlock()
		if ok {
			c.handlerMutex.Lock()
			close(f.closed)
			delete(c.handlers, uint16(id))
			c.handlerMutex.Unlock()
			c.refMutex.Lock()
			c.ref--
			c.refMutex.Unlock()
		}
	}
	c.refMutex.Lock()
	r := c.ref
	c.refMutex.Unlock()
	if r <= 0 {
		baseConnMutex.Lock()
		defer baseConnMutex.Unlock()
		c.close()
		delete(baseConnMap, c.addr)
		ulog.Printf(2, "baseConn(%v): unregister #%d (ref: %d)", c.LocalAddr(), id, c.ref)
	}
}

func (c *baseConn) close() {
	if atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		c.conn.Close()
	}
}

func (c *baseConn) isOpen() bool {
	return atomic.LoadInt32(&c.closed) == 0
}

func (c *baseConn) Send(p *packet) {
	b, err := p.MarshalBinary()
	if err != nil {
		panic(err)
	}
	ulog.Printf(3, "baseConn(%v): SEND: %v to %v", c.LocalAddr(), p, p.addr)
	_, err = c.conn.WriteTo(b, p.addr)
	if err != nil {
		ulog.Printf(3, "%v", err)
		panic(err)
	}
}

func (c *baseConn) RecvSyn(timeout time.Duration) (*packet, error) {
	return c.synPackets.popOne(timeout)
}
