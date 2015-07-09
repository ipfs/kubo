package peerstream_multiplex

import (
	"testing"

	psttest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/test"
)

func TestMultiplexTransport(t *testing.T) {
	psttest.SubtestAll(t, DefaultTransport)
}
