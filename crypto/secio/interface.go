// package secio handles establishing secure communication between two peers.
package secio

import (
	"io"

	ci "github.com/jbenet/go-ipfs/crypto"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"

	peer "github.com/jbenet/go-ipfs/peer"
)

// SessionGenerator constructs secure communication sessions for a peer.
type SessionGenerator struct {
	LocalID    peer.ID
	PrivateKey ci.PrivKey
}

// NewSession takes an insecure io.ReadWriter, performs a TLS-like
// handshake with the other side, and returns a secure session.
// See the source for the protocol details and security implementation.
// The provided Context is only needed for the duration of this function.
func (sg *SessionGenerator) NewSession(ctx context.Context,
	insecure io.ReadWriter) (Session, error) {

	ss, err := newSecureSession(sg.LocalID, sg.PrivateKey)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
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
	LocalPeer() peer.ID

	// LocalPrivateKey retrieves the local private key
	LocalPrivateKey() ci.PrivKey

	// RemotePeer retrieves the remote peer.
	RemotePeer() peer.ID

	// RemotePublicKey retrieves the remote's public key
	// which was received during the handshake.
	RemotePublicKey() ci.PubKey

	// Close closes the secure session
	Close() error
}

// SecureReadWriter returns the encrypted communication channel
func (s *secureSession) ReadWriter() msgio.ReadWriteCloser {
	return s.secure
}

// LocalPeer retrieves the local peer.
func (s *secureSession) LocalPeer() peer.ID {
	return s.localPeer
}

// LocalPrivateKey retrieves the local peer's PrivateKey
func (s *secureSession) LocalPrivateKey() ci.PrivKey {
	return s.localKey
}

// RemotePeer retrieves the remote peer.
func (s *secureSession) RemotePeer() peer.ID {
	return s.remotePeer
}

// RemotePeer retrieves the remote peer.
func (s *secureSession) RemotePublicKey() ci.PubKey {
	return s.remote.permanentPubKey
}

// Close closes the secure session
func (s *secureSession) Close() error {
	return s.secure.Close()
}
