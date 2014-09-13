package bitswap

import (
	"testing"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	bsmsg "github.com/jbenet/go-ipfs/bitswap/message"
	peer "github.com/jbenet/go-ipfs/peer"
)

func TestDoesntPanicIfDelegateNotPresent(t *testing.T) {
	r := receiver{}
	r.ReceiveMessage(context.Background(), &peer.Peer{}, bsmsg.New())
}
