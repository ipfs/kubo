package peerstream_multiplex

import (
	"testing"

	test "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/test"
)

func TestMultiplexTransport(t *testing.T) {
	test.SubtestAll(t, DefaultTransport)
}
