package meterconn

import (
	manet "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr-net"
	metrics "github.com/ipfs/go-ipfs/metrics"
)

type MeteredConn struct {
	mesRecv metrics.MeterCallback
	mesSent metrics.MeterCallback

	manet.Conn
}

func WrapConn(bwc metrics.Reporter, c manet.Conn) manet.Conn {
	return newMeteredConn(c, bwc.LogRecvMessage, bwc.LogSentMessage)
}

func newMeteredConn(base manet.Conn, rcb metrics.MeterCallback, scb metrics.MeterCallback) manet.Conn {
	return &MeteredConn{
		Conn:    base,
		mesRecv: rcb,
		mesSent: scb,
	}
}

func (mc *MeteredConn) Read(b []byte) (int, error) {
	n, err := mc.Conn.Read(b)

	mc.mesRecv(int64(n))
	return n, err
}

func (mc *MeteredConn) Write(b []byte) (int, error) {
	n, err := mc.Conn.Write(b)

	mc.mesSent(int64(n))
	return n, err
}
