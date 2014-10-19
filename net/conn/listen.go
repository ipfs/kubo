package conn

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"

	peer "github.com/jbenet/go-ipfs/peer"
)

// listener is an object that can accept connections. It implements Listener
type listener struct {
	manet.Listener

	// chansize is the size of the internal channels for concurrency
	chansize int

	// channel of incoming conections
	conns chan Conn

	// Local multiaddr to listen on
	maddr ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	local *peer.Peer

	// Peerstore is the set of peers we know about locally
	peers peer.Peerstore

	// Context for children Conn
	ctx context.Context

	// embedded ContextCloser
	ContextCloser
}

// disambiguate
func (l *listener) Close() error {
	return l.ContextCloser.Close()
}

// close called by ContextCloser.Close
func (l *listener) close() error {
	log.Info("listener closing: %s %s", l.local, l.maddr)
	return l.Listener.Close()
}

func (l *listener) listen() {
	l.Children().Add(1)
	defer l.Children().Done()

	// handle at most chansize concurrent handshakes
	sem := make(chan struct{}, l.chansize)

	// handle is a goroutine work function that handles the handshake.
	// it's here only so that accepting new connections can happen quickly.
	handle := func(maconn manet.Conn) {
		defer func() { <-sem }() // release

		c, err := newSingleConn(l.ctx, l.local, nil, maconn)
		if err != nil {
			log.Error("Error accepting connection: %v", err)
			return
		}

		sc, err := newSecureConn(l.ctx, c, l.peers)
		if err != nil {
			log.Error("Error securing connection: %v", err)
			return
		}

		l.conns <- sc
	}

	for {
		maconn, err := l.Listener.Accept()
		if err != nil {

			// if closing, we should exit.
			select {
			case <-l.Closing():
				return // done.
			default:
			}

			log.Error("Failed to accept connection: %v", err)
			continue
		}

		sem <- struct{}{} // acquire
		go handle(maconn)
	}
}

// Accept waits for and returns the next connection to the listener.
// Note that unfortunately this
func (l *listener) Accept() <-chan Conn {
	return l.conns
}

// Multiaddr is the identity of the local Peer.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.maddr
}

// LocalPeer is the identity of the local Peer.
func (l *listener) LocalPeer() *peer.Peer {
	return l.local
}

// Peerstore is the set of peers we know about locally. The Listener needs it
// because when an incoming connection is identified, we should reuse the
// same peer objects (otherwise things get inconsistent).
func (l *listener) Peerstore() peer.Peerstore {
	return l.peers
}

// Listen listens on the particular multiaddr, with given peer and peerstore.
func Listen(ctx context.Context, addr ma.Multiaddr, local *peer.Peer, peers peer.Peerstore) (Listener, error) {

	ml, err := manet.Listen(addr)
	if err != nil {
		return nil, err
	}

	// todo make this a variable
	chansize := 10

	l := &listener{
		Listener: ml,
		maddr:    addr,
		peers:    peers,
		local:    local,
		conns:    make(chan Conn, chansize),
		chansize: chansize,
		ctx:      ctx,
	}

	// need a separate context to use for the context closer.
	// This is because the parent context will be given to all connections too,
	// and if we close the listener, the connections shouldn't share the fate.
	ctx2, _ := context.WithCancel(ctx)
	l.ContextCloser = NewContextCloser(ctx2, l.close)

	go l.listen()

	return l, nil
}
