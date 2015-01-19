// +build darwin freebsd dragonfly netbsd openbsd linux

package reuseport

import (
	"net"
	"os"
	"strconv"
	"syscall"
	"time"

	sockaddrnet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-sockaddr/net"
)

const (
	tcp4       = 52 // "4"
	tcp6       = 54 // "6"
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

	// set non-blocking until after connect, because we cant poll using runtime :(
	// if err = syscall.SetNonblock(fd, true); err != nil {
	// 	syscall.Close(fd)
	// 	return -1, err
	// }

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReuseAddr, 1); err != nil {
		// fmt.Println("reuse addr failed")
		syscall.Close(fd)
		return -1, err
	}

	if err = syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReusePort, 1); err != nil {
		// fmt.Println("reuse port failed")
		syscall.Close(fd)
		return -1, err
	}

	// set setLinger to 5 as reusing exact same (srcip:srcport, dstip:dstport)
	// will otherwise fail on connect.
	if err = setLinger(fd, 5); err != nil {
		// fmt.Println("linger failed")
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
		remoteSockaddr syscall.Sockaddr
		localSockaddr  syscall.Sockaddr
	)

	netAddr, err := ResolveAddr(netw, addr)
	if err != nil {
		// fmt.Println("resolve addr failed")
		return nil, err
	}

	switch netAddr.(type) {
	case *net.TCPAddr, *net.UDPAddr:
	default:
		return nil, ErrUnsupportedProtocol
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

	if fd, err = socket(rfamily, socktype, rprotocol); err != nil {
		return nil, err
	}

	if err = syscall.Bind(fd, localSockaddr); err != nil {
		// fmt.Println("bind failed")
		syscall.Close(fd)
		return nil, err
	}
	if err = connect(fd, remoteSockaddr); err != nil {
		syscall.Close(fd)
		// fmt.Println("connect failed", localSockaddr, err)
		return nil, err
	}

	if rprotocol == syscall.IPPROTO_TCP {
		//  by default golang/net sets TCP no delay to true.
		if err = setNoDelay(fd, true); err != nil {
			// fmt.Println("set no delay failed")
			syscall.Close(fd)
			return nil, err
		}
	}

	if err = syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return nil, err
	}

	switch socktype {
	case syscall.SOCK_STREAM, syscall.SOCK_SEQPACKET:

		// File Name get be nil
		file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
		if c, err = net.FileConn(file); err != nil {
			// fmt.Println("fileconn failed")
			syscall.Close(fd)
			return nil, err
		}

	case syscall.SOCK_DGRAM:

		// File Name get be nil
		file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
		if c, err = net.FileConn(file); err != nil {
			// fmt.Println("fileconn failed")
			syscall.Close(fd)
			return nil, err
		}
	}

	if err = file.Close(); err != nil {
		// fmt.Println("file close failed")
		syscall.Close(fd)
		return nil, err
	}

	return c, err
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
		// fmt.Println("resolve addr failed")
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
		// fmt.Println("bind failed")
		syscall.Close(fd)
		return -1, err
	}

	if protocol == syscall.IPPROTO_TCP {
		//  by default golang/net sets TCP no delay to true.
		if err = setNoDelay(fd, true); err != nil {
			// fmt.Println("set no delay failed")
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
		// fmt.Println("listen failed")
		syscall.Close(fd)
		return nil, err
	}

	file = os.NewFile(uintptr(fd), filePrefix+strconv.Itoa(os.Getpid()))
	if l, err = net.FileListener(file); err != nil {
		// fmt.Println("filelistener failed")
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		// fmt.Println("file close failed")
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
		// fmt.Println("filelistener failed")
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		// fmt.Println("file close failed")
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
		// fmt.Println("filelistener failed")
		syscall.Close(fd)
		return nil, err
	}

	if err = file.Close(); err != nil {
		// fmt.Println("file close failed")
		syscall.Close(fd)
		return nil, err
	}

	return c, err
}

func connect(fd int, ra syscall.Sockaddr) error {
	switch err := syscall.Connect(fd, ra); err {
	case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
	case nil, syscall.EISCONN:
		return nil
	default:
		return err
	}

	var err error
	for {
		// if err := fd.pd.WaitWrite(); err != nil {
		// 	return err
		// }
		// i'd use the above fd.pd.WaitWrite to poll io correctly, just like net sockets...
		// but of course, it uses fucking runtime_* functions that _cannot_ be used by
		// non-go-stdlib source... seriously guys, what kind of bullshit is that!?
		<-time.After(20 * time.Microsecond)
		var nerr int
		nerr, err = syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ERROR)
		if err != nil {
			return err
		}
		switch err = syscall.Errno(nerr); err {
		case syscall.EINPROGRESS, syscall.EALREADY, syscall.EINTR:
		case syscall.Errno(0), syscall.EISCONN:
			return nil
		default:
			return err
		}
	}
}
