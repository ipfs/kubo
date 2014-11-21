package utp

import (
	"errors"
	"net"
	"time"
)

func Dial(n, addr string) (*UTPConn, error) {
	raddr, err := ResolveUTPAddr(n, addr)
	if err != nil {
		return nil, err
	}
	return DialUTP(n, nil, raddr)
}

func DialUTP(n string, laddr, raddr *UTPAddr) (*UTPConn, error) {
	return dial(n, laddr, raddr, 0)
}

func DialUTPTimeout(n string, laddr, raddr *UTPAddr, timeout time.Duration) (*UTPConn, error) {
	return dial(n, laddr, raddr, timeout)
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
func (d *Dialer) Dial(n, addr string) (*UTPConn, error) {
	raddr, err := ResolveUTPAddr(n, addr)
	if err != nil {
		return nil, err
	}

	var laddr *UTPAddr
	if d.LocalAddr != nil {
		var ok bool
		laddr, ok = d.LocalAddr.(*UTPAddr)
		if !ok {
			return nil, errors.New("Dialer.LocalAddr is not a UTPAddr")
		}
	}

	return DialUTPTimeout(n, laddr, raddr, d.Timeout)
}
