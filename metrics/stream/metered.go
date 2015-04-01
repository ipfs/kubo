package meterstream

import (
	metrics "github.com/ipfs/go-ipfs/metrics"
	inet "github.com/ipfs/go-ipfs/p2p/net"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
	protocol "github.com/ipfs/go-ipfs/p2p/protocol"
)

type meteredStream struct {
	// keys for accessing metrics data
	protoKey protocol.ID
	peerKey  peer.ID

	inet.Stream

	// callbacks for reporting bandwidth usage
	mesSent metrics.StreamMeterCallback
	mesRecv metrics.StreamMeterCallback
}

func newMeteredStream(base inet.Stream, pid protocol.ID, p peer.ID, recvCB, sentCB metrics.StreamMeterCallback) inet.Stream {
	return &meteredStream{
		Stream:   base,
		mesSent:  sentCB,
		mesRecv:  recvCB,
		protoKey: pid,
		peerKey:  p,
	}
}

func WrapStream(base inet.Stream, pid protocol.ID, bwc metrics.Reporter) inet.Stream {
	return newMeteredStream(base, pid, base.Conn().RemotePeer(), bwc.LogRecvMessageStream, bwc.LogSentMessageStream)
}

func (s *meteredStream) Read(b []byte) (int, error) {
	n, err := s.Stream.Read(b)

	// Log bytes read
	s.mesRecv(int64(n), s.protoKey, s.peerKey)

	return n, err
}

func (s *meteredStream) Write(b []byte) (int, error) {
	n, err := s.Stream.Write(b)

	// Log bytes written
	s.mesSent(int64(n), s.protoKey, s.peerKey)

	return n, err
}
