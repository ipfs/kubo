package service

import (
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msg "github.com/jbenet/go-ipfs/net/message"
)

type Sender interface {
	SendMessage(ctx context.Context, m msg.NetMessage, rid RequestID) error
	SendRequest(ctx context.Context, m msg.NetMessage) (msg.NetMessage, error)
}
