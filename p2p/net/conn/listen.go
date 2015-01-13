package conn

import (
	"fmt"
	"io"
	"net"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ctxgroup "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-ctxgroup"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	tec "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-temp-err-catcher"
	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"
)

// listener is an object that can accept connections. It implements Listener
type listener struct {
	manet.Listener

	local peer.ID    // LocalPeer is the identity of the local Peer
	privk ic.PrivKey // private key to use to initialize secure conns

	cg ctxgroup.ContextGroup
}

func (l *listener) teardown() error {
	defer log.Debugf("listener closed: %s %s", l.local, l.Multiaddr())
	return l.Listener.Close()
}

func (l *listener) Close() error {
	log.Debugf("listener closing: %s %s", l.local, l.Multiaddr())
	return l.cg.Close()
}

func (l *listener) String() string {
	return fmt.Sprintf("<Listener %s %s>", l.local, l.Multiaddr())
}

// Accept waits for and returns the next connection to the listener.
// Note that unfortunately this
func (l *listener) Accept() (net.Conn, error) {

	// listeners dont have contexts. given changes dont make sense here anymore
	// note that the parent of listener will Close, which will interrupt all io.
	// Contexts and io don't mix.
	ctx := context.Background()

	var catcher tec.TempErrCatcher

	catcher.IsTemp = func(e error) bool {
		// ignore connection breakages up to this point. but log them
		if e == io.EOF {
			log.Debugf("listener ignoring conn with EOF: %s", e)
			return true
		}

		te, ok := e.(tec.Temporary)
		if ok {
			log.Debugf("listener ignoring conn with temporary err: %s", e)
			return te.Temporary()
		}
		return false
	}

	for {
		maconn, err := l.Listener.Accept()
		if err != nil {
			if catcher.IsTemporary(err) {
				continue
			}
			return nil, err
		}

		c, err := newSingleConn(ctx, l.local, "", maconn)
		if err != nil {
			if catcher.IsTemporary(err) {
				continue
			}
			return nil, err
		}

		if l.privk == nil {
			log.Warning("listener %s listening INSECURELY!", l)
			return c, nil
		}
		sc, err := newSecureConn(ctx, l.privk, c)
		if err != nil {
			log.Info("ignoring conn we failed to secure: %s %s", err, sc)
			continue
		}
		return sc, nil
	}
}

func (l *listener) Addr() net.Addr {
	return l.Listener.Addr()
}

// Multiaddr is the identity of the local Peer.
// If there is an error converting from net.Addr to ma.Multiaddr,
// the return value will be nil.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.Listener.Multiaddr()
}

// LocalPeer is the identity of the local Peer.
func (l *listener) LocalPeer() peer.ID {
	return l.local
}

func (l *listener) Loggable() map[string]interface{} {
	return map[string]interface{}{
		"listener": map[string]interface{}{
			"peer":    l.LocalPeer(),
			"address": l.Multiaddr(),
			"secure":  (l.privk != nil),
		},
	}
}

// Listen listens on the particular multiaddr, with given peer and peerstore.
func Listen(ctx context.Context, addr ma.Multiaddr, local peer.ID, sk ic.PrivKey) (Listener, error) {

	ml, err := manet.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("Failed to listen on %s: %s", addr, err)
	}

	l := &listener{
		Listener: ml,
		local:    local,
		privk:    sk,
		cg:       ctxgroup.WithContext(ctx),
	}
	l.cg.SetTeardown(l.teardown)

	log.Debugf("Conn Listener on %s", l.Multiaddr())
	log.Event(ctx, "swarmListen", l)
	return l, nil
}
