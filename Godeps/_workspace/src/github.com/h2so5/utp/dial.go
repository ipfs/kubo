package utp

import (
	"errors"
	"math"
	"math/rand"
	"net"
	"time"
)

// DialUTP connects to the remote address raddr on the network net,
// which must be "utp", "utp4", or "utp6".  If laddr is not nil, it is
// used as the local address for the connection.
func DialUTP(n string, laddr, raddr *Addr) (*Conn, error) {
	return DialUTPTimeout(n, laddr, raddr, 0)
}

// DialUTPTimeout acts like Dial but takes a timeout.
// The timeout includes name resolution, if required.
func DialUTPTimeout(n string, laddr, raddr *Addr, timeout time.Duration) (*Conn, error) {
	conn, err := getSharedBaseConn(n, laddr)
	if err != nil {
		return nil, err
	}

	id := uint16(rand.Intn(math.MaxUint16))
	c := newConn()
	c.conn = conn
	c.raddr = raddr.Addr
	c.rid = id
	c.sid = id + 1
	c.seq = 1
	c.state = stateSynSent
	c.sendbuf = newPacketBuffer(windowSize*2, 1)
	c.conn.Register(int32(c.rid), c.recv)
	go c.loop()
	c.synch <- 0

	t := time.NewTimer(timeout)
	defer t.Stop()
	if timeout == 0 {
		t.Stop()
	}

	select {
	case <-c.connch:
	case <-t.C:
		c.Close()
		return nil, &net.OpError{
			Op:   "dial",
			Net:  c.LocalAddr().Network(),
			Addr: c.LocalAddr(),
			Err:  errTimeout,
		}
	}
	return c, nil
}

// A Dialer contains options for connecting to an address.
//
// The zero value for each field is equivalent to dialing without
// that option. Dialing with the zero value of Dialer is therefore
// equivalent to just calling the Dial function.
type Dialer struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. If Deadline is also set, it may fail
	// earlier.
	//
	// The default is no timeout.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	Timeout time.Duration

	// LocalAddr is the local address to use when dialing an
	// address. The address must be of a compatible type for the
	// network being dialed.
	// If nil, a local address is automatically chosen.
	LocalAddr net.Addr
}

// Dial connects to the address on the named network.
//
// See func Dial for a description of the network and address parameters.
func (d *Dialer) Dial(n, addr string) (*Conn, error) {
	raddr, err := ResolveAddr(n, addr)
	if err != nil {
		return nil, err
	}

	var laddr *Addr
	if d.LocalAddr != nil {
		var ok bool
		laddr, ok = d.LocalAddr.(*Addr)
		if !ok {
			return nil, errors.New("Dialer.LocalAddr is not a Addr")
		}
	}

	return DialUTPTimeout(n, laddr, raddr, d.Timeout)
}
