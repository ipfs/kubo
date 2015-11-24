package multistream

import (
	"testing"

	test "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-stream-muxer/test"
)

func TestMultiStreamTransport(t *testing.T) {
	test.SubtestAll(t, NewTransport())
}
