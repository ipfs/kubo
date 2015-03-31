package peerstream_yamux

import (
	"testing"

	psttest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-peerstream/transport/test"
)

func TestYamuxTransport(t *testing.T) {
	psttest.SubtestAll(t, DefaultTransport)
}
