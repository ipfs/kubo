package spipe

import (
	"errors"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	peer "github.com/jbenet/go-ipfs/peer"
)

// Duplex is a simple duplex channel
type Duplex struct {
	In  chan []byte
	Out chan []byte
}

// SecurePipe objects represent a bi-directional message channel.
type SecurePipe struct {
	Duplex
	insecure Duplex

	local  *peer.Peer
	remote *peer.Peer

	params params

	ctx    context.Context
	cancel context.CancelFunc
}

// options in a secure pipe
type params struct {
}

// NewSecurePipe constructs a pipe with channels of a given buffer size.
func NewSecurePipe(ctx context.Context, bufsize int, local,
	remote *peer.Peer) (*SecurePipe, error) {

	sp := &SecurePipe{
		Duplex: Duplex{
			In:  make(chan []byte, bufsize),
			Out: make(chan []byte, bufsize),
		},
		local:  local,
		remote: remote,
	}
	return sp, nil
}

// Wrap creates a secure connection on top of an insecure duplex channel.
func (s *SecurePipe) Wrap(ctx context.Context, insecure Duplex) error {
	if s.ctx != nil {
		return errors.New("Pipe in use")
	}

	s.insecure = insecure
	s.ctx, s.cancel = context.WithCancel(ctx)

	if err := s.handshake(); err != nil {
		s.cancel()
		return err
	}

	return nil
}

// Close closes the secure pipe
func (s *SecurePipe) Close() error {
	if s.cancel == nil {
		return errors.New("pipe already closed")
	}

	s.cancel()
	s.cancel = nil
	close(s.In)
	return nil
}
