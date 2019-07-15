package main

import (
	"context"
	"errors"
	"fmt"
	"gx/ipfs/QmZcLBXKaFe8ND5YHPkJRAwmhJGrVsi1JqDZNyJ4nRK5Mj/go-multiaddr-net"
	"io"
	"net"
	"syscall/js"
	"time"

	ma "gx/ipfs/QmNTCey11oxhb1AxDnQBRHtdhap6Ctud872NjAYPYYXPuc/go-multiaddr"
	"gx/ipfs/QmPJxxDsX2UbchSHobbYuvz7qnyJTFKvaKMzE2rZWJ4x5B/go-libp2p-peer"
	tptu "gx/ipfs/QmQwvsMzMDTW2K8ySZYgnTVCkzQXVDxmGB5upvVFwdumJV/go-libp2p-transport-upgrader"
	ws "gx/ipfs/QmTPHQHPgmoJtmhMSpVhRPeb23kXGgkUEDKfn9ZndnaQmy/go-ws-transport"
	"gx/ipfs/QmUDtgnEr7FFrtK2LQM2dFzTNWghnrApBDcU3iHEJz8eQS/go-libp2p-transport"
)

const (
	wsConnecting = iota
	wsOpen
	wsClosing
	wsClosed
)

type jsws struct {
	Upgrader *tptu.Upgrader
}

type jsconn struct {
	ready chan struct{}

	raddr ma.Multiaddr
	wsock js.Value

	readR *io.PipeReader
	readW *io.PipeWriter

	cb js.Callback
}

func (c *jsconn) Read(b []byte) (n int, err error) {
	return c.readR.Read(b)
}

func (c *jsconn) onmessage(value []js.Value) {
	u8a := js.Global().Get("Uint8Array").New(value[0].Get("data"))

	// there is, very likely, a much, much better way
	buf := make([]byte, u8a.Length())
	for i := range buf {
		buf[i] = byte(u8a.Index(i).Int())
	}

	if _, err := c.readW.Write(buf); err != nil {
		panic(err)
	}
}

func (c *jsconn) Write(b []byte) (n int, err error) {
	<-c.ready
	arr := js.TypedArrayOf(b)
	defer arr.Release()
	c.wsock.Call("send", arr)
	return len(b), err
}

func (c *jsconn) LocalAddr() net.Addr {
	a, _ := manet.ToNetAddr(c.LocalMultiaddr())
	return a //TODO: probably broken
}

func (c *jsconn) RemoteAddr() net.Addr {
	a, _ := manet.ToNetAddr(c.raddr)
	return a //TODO: probably broken
}

func (c *jsconn) SetDeadline(t time.Time) error {
	return nil
}

func (c *jsconn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *jsconn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *jsconn) LocalMultiaddr() ma.Multiaddr {
	m, _ := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/80/ws")
	return m
}

func (c *jsconn) RemoteMultiaddr() ma.Multiaddr {
	return c.raddr
}

func (c *jsconn) Close() error {
	c.wsock.Call("close")
	c.cb.Release()
	return nil // todo: errors?
}

func NewJsWs(u *tptu.Upgrader) *jsws {
	return &jsws{u}
}

func (t *jsws) CanDial(addr ma.Multiaddr) bool {
	return ws.WsFmt.Matches(addr)
}

func (t *jsws) Protocols() []int {
	return []int{ws.WsProtocol.Code}
}

func (t *jsws) Proxy() bool {
	return false
}

func (t *jsws) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (transport.Conn, error) {
	var addr = ""
	var err error

	ps := raddr.Protocols()
	if len(ps) != 3 {
		return nil, fmt.Errorf("unexpected protocol count")
	}

	switch ps[0].Code {
	case ma.P_IP6:
		addr, err = raddr.ValueForProtocol(ps[0].Code)
		if err != nil {
			return nil, err
		}
		addr = fmt.Sprintf("[%s]", addr)
	case ma.P_IP4:
		addr, err = raddr.ValueForProtocol(ps[0].Code)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported proto %d", ps[0].Code)
	}

	switch ps[1].Code {
	case ma.P_TCP:
		port, err := raddr.ValueForProtocol(ps[1].Code)
		if err != nil {
			return nil, err
		}
		addr = fmt.Sprintf("%s:%s", addr, port)
	default:
		return nil, fmt.Errorf("unsupported proto %d", ps[0].Code)
	}

	switch ps[2].Code {
	case ws.WsProtocol.Code:
		addr = "ws://" + addr
	default:
		return nil, fmt.Errorf("unsupported proto %d", ps[0].Code)
	}
	println("connecting to " + addr)

	wsock := js.Global().Get("WebSocket").New(addr)
	rr, rw := io.Pipe()
	ready := make(chan struct{})

	conn := &jsconn{
		ready: ready,

		raddr: raddr,
		wsock: wsock,

		readR: rr,
		readW: rw,
	}
	conn.cb = js.NewCallback(conn.onmessage)
	opencb := js.NewCallback(func(args []js.Value) {
		println("onopen")
		close(conn.ready)
	})
	go func() {
		<-conn.ready
		opencb.Release()
	}()
	var closecb js.Callback
	closecb = js.NewCallback(func(args []js.Value) {
		println("onclose")
		_ = conn.readR.Close()
		closecb.Release()
	})

	wsock.Set("binaryType", "arraybuffer")
	wsock.Set("onmessage", conn.cb)
	wsock.Set("onopen", opencb)
	wsock.Set("onclose", closecb)

	return t.Upgrader.UpgradeOutbound(ctx, t, conn, p)
}

func (t *jsws) Listen(laddr ma.Multiaddr) (transport.Listener, error) {
	return nil, errors.New("not supported")
}
