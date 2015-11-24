package sm_yamux

import (
	"testing"

	test "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/test"
)

func TestYamuxTransport(t *testing.T) {
	test.SubtestAll(t, DefaultTransport)
}
