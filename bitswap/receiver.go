package bitswap

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	bsnet "github.com/jbenet/go-ipfs/bitswap/network"
	peer "github.com/jbenet/go-ipfs/peer"
)

// receiver breaks the circular dependency between bitswap and its sender
// NB: A sender is instantiated with a handler and this sender is then passed
// as a constructor argument to BitSwap. However, the handler is BitSwap!
// Hence, this receiver.
type receiver struct {
	delegate bsnet.Receiver
}

func (r *receiver) ReceiveMessage(
	ctx context.Context, incoming bsmsg.BitSwapMessage) (
	bsmsg.BitSwapMessage, *peer.Peer, error) {
	if r.delegate == nil {
		return nil, nil, nil
	}
	return r.delegate.ReceiveMessage(ctx, incoming)
}

func (r *receiver) Delegate(delegate bsnet.Receiver) {
	r.delegate = delegate
}
