package multistream

import (
	"testing"

	psttest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/test"
)

func TestMultiStreamTransport(t *testing.T) {
	psttest.SubtestAll(t, NewTransport())
}
