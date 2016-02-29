package peerstream_spdystream

import (
	"testing"

	test "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/test"
)

func TestSpdyStreamTransport(t *testing.T) {
	test.SubtestAll(t, Transport)
}
