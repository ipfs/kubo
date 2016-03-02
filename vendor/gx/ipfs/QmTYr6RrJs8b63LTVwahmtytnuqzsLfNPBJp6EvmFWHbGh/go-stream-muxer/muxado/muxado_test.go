package peerstream_muxado

import (
	"testing"

	test "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/test"
)

func TestMuxadoTransport(t *testing.T) {
	test.SubtestAll(t, Transport)
}
