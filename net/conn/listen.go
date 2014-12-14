package conn

import (
	"fmt"
	"net"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"

	peer "github.com/jbenet/go-ipfs/peer"
)

// listener is an object that can accept connections. It implements Listener
type listener struct {
	notSecure        bool
	notSecureIMeanIt bool

	manet.Listener

	// Local multiaddr to listen on
	maddr ma.Multiaddr

	// LocalPeer is the identity of the local Peer.
	local peer.Peer

	// Peerstore is the set of peers we know about locally
	peers peer.Peerstore
}

func (l *listener) Close() error {
	log.Infof("listener closing: %s %s", l.local, l.maddr)
	return l.Listener.Close()
}

// Accept waits for and returns the next connection to the listener.
// Note that unfortunately this
func (l *listener) Accept() (net.Conn, error) {

	// listeners dont have contexts. given changes dont make sense here anymore
	// note that the parent of listener will Close, which will interrupt all io.
	// Contexts and io don't mix.
	ctx := context.Background()

	maconn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}

	c, err := newSingleConn(ctx, l.local, nil, maconn)
	if err != nil {
		return nil, fmt.Errorf("Error accepting connection: %v", err)
	}

	if l.Secure() {
		sc, err := newSecureConn(ctx, c, l.peers)
		if err != nil {
			return nil, fmt.Errorf("Error securing connection: %v", err)
		}
		return sc, nil
	}

	return c, nil
}

func (l *listener) Secure() bool {
	return !(l.notSecure && l.notSecureIMeanIt)
}

func (l *listener) Addr() net.Addr {
	return l.Listener.Addr()
}

// Multiaddr is the identity of the local Peer.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.maddr
}

// LocalPeer is the identity of the local Peer.
func (l *listener) LocalPeer() peer.Peer {
	return l.local
}

// Peerstore is the set of peers we know about locally. The Listener needs it
// because when an incoming connection is identified, we should reuse the
// same peer objects (otherwise things get inconsistent).
func (l *listener) Peerstore() peer.Peerstore {
	return l.peers
}

func (l *listener) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"listener": map[string]interface{}{
			"peer":    l.LocalPeer(),
			"address": l.Multiaddr(),
			"secure":  l.Secure(),
		},
	}
}

// Listen listens on the particular multiaddr, with given peer and peerstore.
func Listen(ctx context.Context, addr ma.Multiaddr, local peer.Peer, peers peer.Peerstore) (Listener, error) {

	ml, err := manet.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on %s: %s", addr, err)
	}

	l := &listener{
		Listener:         ml,
		maddr:            addr,
		peers:            peers,
		local:            local,
		notSecure:        false,
		notSecureIMeanIt: false,
	}

	log.Infof("swarm listening on %s\n", l.Multiaddr())
	log.Event(ctx, "swarmListen", l)
	return l, nil
}
