// package main provides an implementation of netcat using the secio package.
// This means the channel is encrypted (and MACed).
// It is meant to exercise the spipe package.
// Usage:
//    seccat [<local address>] <remote address>
//    seccat -l <local address>
//
// Address format is: [host]:port
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	logging "github.com/ipfs/go-log"
	ci "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	pstore "github.com/libp2p/go-libp2p-core/peerstore"
	pstoremem "github.com/libp2p/go-libp2p-peerstore/pstoremem"
	secio "github.com/libp2p/go-libp2p-secio"
)

var verbose = false

// Usage prints out the usage of this module.
// Assumes flags use go stdlib flag package.
var Usage = func() {
	text := `seccat - secure netcat in Go

Usage:

  listen: %s [<local address>] <remote address>
  dial:   %s -l <local address>

Address format is Go's: [host]:port
`

	fmt.Fprintf(os.Stderr, text, os.Args[0], os.Args[0])
	flag.PrintDefaults()
}

type args struct {
	listen     bool
	verbose    bool
	debug      bool
	localAddr  string
	remoteAddr string
	// keyfile    string
	keybits int
}

func parseArgs() args {
	var a args

	// setup + parse flags
	flag.BoolVar(&a.listen, "listen", false, "listen for connections")
	flag.BoolVar(&a.listen, "l", false, "listen for connections (short)")
	flag.BoolVar(&a.verbose, "v", true, "verbose")
	flag.BoolVar(&a.debug, "debug", false, "debugging")
	// flag.StringVar(&a.keyfile, "key", "", "private key file")
	flag.IntVar(&a.keybits, "keybits", 2048, "num bits for generating private key")
	flag.Usage = Usage
	flag.Parse()
	osArgs := flag.Args()

	if len(osArgs) < 1 {
		exit("")
	}

	if a.verbose {
		out("verbose on")
	}

	if a.listen {
		a.localAddr = osArgs[0]
	} else {
		if len(osArgs) > 1 {
			a.localAddr = osArgs[0]
			a.remoteAddr = osArgs[1]
		} else {
			a.remoteAddr = osArgs[0]
		}
	}

	return a
}

func main() {
	args := parseArgs()
	verbose = args.verbose
	if args.debug {
		logging.SetDebugLogging()
	}

	go func() {
		// wait until we exit.
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGABRT)
		<-sigc
		panic("ABORT! ABORT! ABORT!")
	}()

	if err := connect(args); err != nil {
		exit("%s", err)
	}
}

func setupPeer(a args) (peer.ID, pstore.Peerstore, error) {
	if a.keybits < ci.MinRsaKeyBits {
		return "", nil, ci.ErrRsaKeyTooSmall
	}

	out("generating key pair...")
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, a.keybits)
	if err != nil {
		return "", nil, err
	}

	p, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return "", nil, err
	}

	ps := pstoremem.NewPeerstore()
	err = ps.AddPrivKey(p, sk)
	if err != nil {
		return "", nil, err
	}
	err = ps.AddPubKey(p, pk)
	if err != nil {
		return "", nil, err
	}

	out("local peer id: %s", p)
	return p, ps, nil
}

func connect(args args) error {
	p, ps, err := setupPeer(args)
	if err != nil {
		return err
	}

	var conn net.Conn
	if args.listen {
		conn, err = Listen(args.localAddr)
	} else {
		conn, err = Dial(args.localAddr, args.remoteAddr)
	}
	if err != nil {
		return err
	}

	// log everything that goes through conn
	rwc := &logConn{n: "conn", Conn: conn}

	// OK, let's setup the channel.
	sk := ps.PrivKey(p)
	sg, err := secio.New(sk)
	if err != nil {
		return err
	}
	sconn, err := sg.SecureInbound(context.TODO(), rwc)
	if err != nil {
		return err
	}
	out("remote peer id: %s", sconn.RemotePeer())
	netcat(sconn)
	return nil
}

// Listen listens and accepts one incoming UDT connection on a given port,
// and pipes all incoming data to os.Stdout.
func Listen(localAddr string) (net.Conn, error) {
	l, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	out("listening at %s", l.Addr())

	c, err := l.Accept()
	if err != nil {
		return nil, err
	}
	out("accepted connection from %s", c.RemoteAddr())

	// done with listener
	l.Close()

	return c, nil
}

// Dial connects to a remote address and pipes all os.Stdin to the remote end.
// If localAddr is set, uses it to Dial from.
func Dial(localAddr, remoteAddr string) (net.Conn, error) {

	var laddr net.Addr
	var err error
	if localAddr != "" {
		laddr, err = net.ResolveTCPAddr("tcp", localAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve address %s", localAddr)
		}
	}

	if laddr != nil {
		out("dialing %s from %s", remoteAddr, laddr)
	} else {
		out("dialing %s", remoteAddr)
	}

	d := net.Dialer{LocalAddr: laddr}
	c, err := d.Dial("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}
	out("connected to %s", c.RemoteAddr())

	return c, nil
}

func netcat(c io.ReadWriteCloser) {
	out("piping stdio to connection")

	done := make(chan struct{}, 2)

	go func() {
		n, _ := io.Copy(c, os.Stdin)
		out("sent %d bytes", n)
		done <- struct{}{}
	}()
	go func() {
		n, _ := io.Copy(os.Stdout, c)
		out("received %d bytes", n)
		done <- struct{}{}
	}()

	// wait until we exit.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, notifySignals...)

	select {
	case <-done:
	case <-sigc:
		return
	}

	c.Close()
}
