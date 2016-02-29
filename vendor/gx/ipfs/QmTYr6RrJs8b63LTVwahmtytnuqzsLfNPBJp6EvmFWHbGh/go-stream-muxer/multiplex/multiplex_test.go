package peerstream_multiplex

import (
	"testing"

	test "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/test"
)

func TestMultiplexTransport(t *testing.T) {
	test.SubtestAll(t, DefaultTransport)
}
