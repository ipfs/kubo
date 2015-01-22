// +build darwin freebsd dragonfly netbsd openbsd linux

package reuseport

import (
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	poll "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport/poll"
	sockaddrnet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-sockaddr/net"
)

const (
	filePrefix = "port."
)

// Wrapper around the socket system call that marks the returned file
// descriptor as nonblocking and close-on-exec.
func socket(family, socktype, protocol int) (fd int, err error) {
	syscall.ForkLock.RLock()
	fd, err = syscall.Socket(family, socktype, protocol)
	if err == nil {
		syscall.CloseOnExec(fd)
	}
	syscall.ForkLock.RUnlock()

	if err != nil {
		return -1, err
	}

	// cant set it until after connect
	// if err = syscall.SetNonblock(fd, true); err != nil {
	// 	syscall.Close(fd)
	// 	return -1, err
	// }

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReuseAddr, 1); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReusePort, 1); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	// set setLinger to 5 as reusing exact same (srcip:srcport, dstip:dstport)
	// will otherwise fail on connect.
	if err = setLinger(fd, 5); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

func dial(dialer net.Dialer, netw, addr string) (c net.Conn, err error) {
	var (
		fd             int
		lfamily        int
		rfamily        int
		socktype       int
		lprotocol      int
		rprotocol      int
		file           *os.File
		deadline       time.Time
		remoteSockaddr syscall.Sockaddr
		localSockaddr  syscall.Sockaddr
	)

	netAddr, err := ResolveAddr(netw, addr)
	if err != nil {
		return nil, err
	}

	switch netAddr.(type) {
	case *net.TCPAddr, *net.UDPAddr:
	default:
		return nil, ErrUnsupportedProtocol
	}

	switch {
	case !dialer.Deadline.IsZero():
		deadline = dialer.Deadline
	case dialer.Timeout != 0:
		deadline = time.Now().Add(dialer.Timeout)
	}

	localSockaddr = sockaddrnet.NetAddrToSockaddr(dialer.LocalAddr)
	remoteSockaddr = sockaddrnet.NetAddrToSockaddr(netAddr)

	rfamily = sockaddrnet.NetAddrAF(netAddr)
	rprotocol = sockaddrnet.NetAddrIPPROTO(netAddr)
	socktype = sockaddrnet.NetAddrSOCK(netAddr)

	if dialer.LocalAddr != nil {
		switch dialer.LocalAddr.(type) {
		case *net.TCPAddr, *net.UDPAddr:
		default:
			return nil, ErrUnsupportedProtocol
		}

		// check family and protocols match.
		lfamily = sockaddrnet.NetAddrAF(dialer.LocalAddr)
		lprotocol = sockaddrnet.NetAddrIPPROTO(dialer.LocalAddr)
		if lfamily != rfamily && lprotocol != rfamily {
			return nil, &net.AddrError{Err: "unexpected address type", Addr: netAddr.String()}
		}
	}

	// look at dialTCP in http://golang.org/src/net/tcpsock_posix.go  .... !
	// here we just try again 3 times.
	for i := 0; i < 3; i++ {
		if !deadline.IsZero() && deadline.Before(time.Now()) {
			err = errTimeout
			break
		}

		if fd, err = socket(rfamily, socktype, rprotocol); err != nil {
			return nil, err
		}

		if localSockaddr != nil {
			if err = syscall.Bind(fd, localSockaddr); err != nil {
				syscall.Close(fd)
				return nil, err
			}
		}

		if err = syscall.SetNonblock(fd, true); err != nil {
			syscall.Close(fd)
			return nil, err
		}

		if err = connect(fd, remoteSockaddr, deadline); err != nil {
			syscall.Close(fd)
			continue // try again.
		}

		break
	}
	if err != nil {
		return nil, err
	}

	if rprotocol == syscall.IPPROTO_TCP {
		//  by default golang/net sets TCP no delay to true.
		if err = setNoDelay(fd, true); err != nil {
			syscall.Close(fd)
			return nil, err
		}
	}

	// File Name get be nil
	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if c, err = net.FileConn(file); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	// c = wrapConnWithRemoteAddr(c, netAddr)
	return c, err
}

// there's a rare case where dial returns successfully but for some reason the
// RemoteAddr is not yet set. So, since we know what raddr should be, we just
// wrap it. This is not ideal in that sometimes getpeername() may return a
// different addr. But until this is fixed, best way to do it.
//  * https://gist.github.com/jbenet/5c191d698fe9ec58c49d
//  * https://github.com/golang/go/issues/9661#issuecomment-71043147
func wrapConnWithRemoteAddr(c net.Conn, raddr net.Addr) net.Conn {
	if c.RemoteAddr() == nil {
		return &conn{Conn: c, raddr: raddr}
	}
	return c // it's fine, no need to wrap.
}

func listen(netw, addr string) (fd int, err error) {
	var (
		family   int
		socktype int
		protocol int
		sockaddr syscall.Sockaddr
	)

	netAddr, err := ResolveAddr(netw, addr)
	if err != nil {
		return -1, err
	}

	switch netAddr.(type) {
	case *net.TCPAddr, *net.UDPAddr:
	default:
		return -1, ErrUnsupportedProtocol
	}

	family = sockaddrnet.NetAddrAF(netAddr)
	protocol = sockaddrnet.NetAddrIPPROTO(netAddr)
	sockaddr = sockaddrnet.NetAddrToSockaddr(netAddr)
	socktype = sockaddrnet.NetAddrSOCK(netAddr)

	if fd, err = socket(family, socktype, protocol); err != nil {
		return -1, err
	}

	if err = syscall.Bind(fd, sockaddr); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	if protocol == syscall.IPPROTO_TCP {
		//  by default golang/net sets TCP no delay to true.
		if err = setNoDelay(fd, true); err != nil {
			syscall.Close(fd)
			return -1, err
		}
	}

	if err = syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return -1, err
	}

	return fd, nil
}

func listenStream(netw, addr string) (l net.Listener, err error) {
	var (
		file *os.File
	)

	fd, err := listen(netw, addr)
	if err != nil {
		return nil, err
	}

	// Set backlog size to the maximum
	if err = syscall.Listen(fd, syscall.SOMAXCONN); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if l, err = net.FileListener(file); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return l, err
}

func listenPacket(netw, addr string) (p net.PacketConn, err error) {
	var (
		file *os.File
	)

	fd, err := listen(netw, addr)
	if err != nil {
		return nil, err
	}

	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if p, err = net.FilePacketConn(file); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return p, err
}

func listenUDP(netw, addr string) (c net.Conn, err error) {
	var (
		file *os.File
	)

	fd, err := listen(netw, addr)
	if err != nil {
		return nil, err
	}

	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if c, err = net.FileConn(file); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	return c, err
}

// this is close to the connect() function inside stdlib/net
func connect(fd int, ra syscall.Sockaddr, deadline time.Time) error {
	switch err := syscall.Connect(fd, ra); err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil, syscall.EISCONN:
		if !deadline.IsZero() && deadline.Before(time.Now()) {
			return errTimeout
		}
		return nil
	default:
		return err
	}

	poller, err := poll.New(fd)
	if err != nil {
		return err
	}

	for {
		if err = poller.WaitWrite(deadline); err != nil {
			return err
		}

		// if err := fd.pd.WaitWrite(); err != nil {
		// 	return err
		// }
		// i'd use the above fd.pd.WaitWrite to poll io correctly, just like net sockets...
		// but of course, it uses the damn runtime_* functions that _cannot_ be used by
		// non-go-stdlib source... seriously guys, this is not nice.
		// we're relegated to using syscall.Select (what nightmare that is) or using
		// a simple but totally bogus time-based wait. such garbage.
		var nerr int
		nerr, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return err
		}
		switch err = syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
			continue
		case syscall.Errno(0), syscall.EISCONN:
			if !deadline.IsZero() && deadline.Before(time.Now()) {
				return errTimeout
			}
			return nil
		default:
			return err
		}
	}
}

var errTimeout = &timeoutError{}

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "i/o timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }
