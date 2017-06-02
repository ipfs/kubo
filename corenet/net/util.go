package net

import (
	"io"

	corenet "github.com/ipfs/go-ipfs/corenet"
)

func startStreaming(stream *corenet.StreamInfo) {
	go func() {
		io.Copy(stream.Local, stream.Remote)
		stream.Close()
	}()

	go func() {
		io.Copy(stream.Remote, stream.Local)
		stream.Close()
	}()
}
