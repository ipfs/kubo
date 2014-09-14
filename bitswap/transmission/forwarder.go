package transmission

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

// Forwarder breaks the circular dependency between bitswap and its sender
// NB: A sender is instantiated with a handler and this sender is then passed
// as a constructor argument to BitSwap. However, the handler is BitSwap!
// Hence, this receiver.
type Forwarder struct {
	delegate Receiver
}

func (r *Forwarder) ReceiveMessage(
	ctx context.Context, sender *peer.Peer, incoming bsmsg.BitSwapMessage) (
	bsmsg.BitSwapMessage, *peer.Peer, error) {
	if r.delegate == nil {
		return nil, nil, nil
	}
	return r.delegate.ReceiveMessage(ctx, sender, incoming)
}

func (r *Forwarder) Delegate(delegate Receiver) {
	r.delegate = delegate
}
