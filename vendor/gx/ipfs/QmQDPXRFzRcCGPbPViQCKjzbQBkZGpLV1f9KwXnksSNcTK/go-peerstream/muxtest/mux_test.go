package muxtest

import (
	"testing"

	multiplex "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/multiplex"
	multistream "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/multistream"
	muxado "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/muxado"
	spdy "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/spdystream"
	yamux "gx/ipfs/QmTYr6RrJs8b63LTVwahmtytnuqzsLfNPBJp6EvmFWHbGh/go-stream-muxer/yamux"
)

func TestYamuxTransport(t *testing.T) {
	SubtestAll(t, yamux.DefaultTransport)
}

func TestSpdyStreamTransport(t *testing.T) {
	SubtestAll(t, spdy.Transport)
}

func TestMultiplexTransport(t *testing.T) {
	SubtestAll(t, multiplex.DefaultTransport)
}

func TestMuxadoTransport(t *testing.T) {
	SubtestAll(t, muxado.Transport)
}

func TestMultistreamTransport(t *testing.T) {
	SubtestAll(t, multistream.NewTransport())
}
