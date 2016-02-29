package multistream

import (
	"testing"

	test "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/test"
)

func TestMultiStreamTransport(t *testing.T) {
	test.SubtestAll(t, NewTransport())
}
