// package secio handles establishing secure communication between two peers.
package secio

import (
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"

	peer "github.com/jbenet/go-ipfs/peer"
)

// SessionGenerator constructs secure communication sessions for a peer.
type SessionGenerator struct {
	Local     peer.Peer
	Peerstore peer.Peerstore
}

// NewSession takes an insecure io.ReadWriter, performs a TLS-like
// handshake with the other side, and returns a secure session.
// See the source for the protocol details and security implementation.
// The provided Context is only needed for the duration of this function.
func (sg *SessionGenerator) NewSession(ctx context.Context,
	insecure io.ReadWriter) (Session, error) {

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	ss := newSecureSession(sg.Local, sg.Peerstore)
	if err := ss.handshake(ctx, insecure); err != nil {
		cancel()
		return nil, err
	}

	return ss, nil
}

type Session interface {
	// ReadWriter returns the encrypted communication channel
	ReadWriter() msgio.ReadWriteCloser

	// LocalPeer retrieves the local peer.
	LocalPeer() peer.Peer

	// RemotePeer retrieves the local peer.
	RemotePeer() peer.Peer

	// Close closes the secure session
	Close() error
}

// SecureReadWriter returns the encrypted communication channel
func (s *secureSession) ReadWriter() msgio.ReadWriteCloser {
	return s.secure
}

// LocalPeer retrieves the local peer.
func (s *secureSession) LocalPeer() peer.Peer {
	return s.localPeer
}

// RemotePeer retrieves the local peer.
func (s *secureSession) RemotePeer() peer.Peer {
	return s.remotePeer
}

// Close closes the secure session
func (s *secureSession) Close() error {
	return s.secure.Close()
}
