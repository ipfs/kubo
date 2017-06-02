package net

import (
	"io"

	ptp "github.com/ipfs/go-ipfs/ptp"
)

func startStreaming(stream *ptp.StreamInfo) {
	go func() {
		io.Copy(stream.Local, stream.Remote)
		stream.Close()
	}()

	go func() {
		io.Copy(stream.Remote, stream.Local)
		stream.Close()
	}()
}
