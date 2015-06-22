package conn

import (
	"fmt"
	"io"
	"net"

	ma "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	reuseport "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-reuseport"
	tec "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-temp-err-catcher"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess"
	goprocessctx "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/goprocess/context"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	ic "github.com/ipfs/go-ipfs/p2p/crypto"
	filter "github.com/ipfs/go-ipfs/p2p/net/filter"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

// ConnWrapper is any function that wraps a raw multiaddr connection
type ConnWrapper func(manet.Conn) manet.Conn

// listener is an object that can accept connections. It implements Listener
type listener struct {
	manet.Listener

	local peer.ID    // LocalPeer is the identity of the local Peer
	privk ic.PrivKey // private key to use to initialize secure conns

	filters *filter.Filters

	wrapper ConnWrapper

	proc goprocess.Process
}

func (l *listener) teardown() error {
	defer log.Debugf("listener closed: %s %s", l.local, l.Multiaddr())
	return l.Listener.Close()
}

func (l *listener) Close() error {
	log.Debugf("listener closing: %s %s", l.local, l.Multiaddr())
	return l.proc.Close()
}

func (l *listener) String() string {
	return fmt.Sprintf("<Listener %s %s>", l.local, l.Multiaddr())
}

func (l *listener) SetAddrFilters(fs *filter.Filters) {
	l.filters = fs
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

		log.Debugf("listener %s got connection: %s <---> %s", l, maconn.LocalMultiaddr(), maconn.RemoteMultiaddr())

		if l.filters != nil && l.filters.AddrBlocked(maconn.RemoteMultiaddr()) {
			log.Debugf("blocked connection from %s", maconn.RemoteMultiaddr())
			maconn.Close()
			continue
		}
		// If we have a wrapper func, wrap this conn
		if l.wrapper != nil {
			maconn = l.wrapper(maconn)
		}

		c, err := newSingleConn(ctx, l.local, "", maconn)
		if err != nil {
			if catcher.IsTemporary(err) {
				continue
			}
			return nil, err
		}

		if l.privk == nil || EncryptConnections == false {
			log.Warning("listener %s listening INSECURELY!", l)
			return c, nil
		}
		sc, err := newSecureConn(ctx, l.privk, c)
		if err != nil {
			log.Infof("ignoring conn we failed to secure: %s %s", err, c)
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
	ml, err := manetListen(addr)
	if err != nil {
		return nil, err
	}

	l := &listener{
		Listener: ml,
		local:    local,
		privk:    sk,
	}
	l.proc = goprocessctx.WithContextAndTeardown(ctx, l.teardown)

	log.Debugf("Conn Listener on %s", l.Multiaddr())
	log.Event(ctx, "swarmListen", l)
	return l, nil
}

type ListenerConnWrapper interface {
	SetConnWrapper(ConnWrapper)
}

// SetConnWrapper assigns a maconn ConnWrapper to wrap all incoming
// connections with. MUST be set _before_ calling `Accept()`
func (l *listener) SetConnWrapper(cw ConnWrapper) {
	l.wrapper = cw
}

func manetListen(addr ma.Multiaddr) (manet.Listener, error) {
	network, naddr, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}

	if reuseportIsAvailable() {
		nl, err := reuseport.Listen(network, naddr)
		if err == nil {
			// hey, it worked!
			return manet.WrapNetListener(nl)
		}
		// reuseport is available, but we failed to listen. log debug, and retry normally.
		log.Debugf("reuseport available, but failed to listen: %s %s, %s", network, naddr, err)
	}

	// either reuseport not available, or it failed. try normally.
	return manet.Listen(addr)
}
