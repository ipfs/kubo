package conn

import (
	"fmt"
	"io"

	handshake "github.com/jbenet/go-ipfs/net/handshake"
	hspb "github.com/jbenet/go-ipfs/net/handshake/pb"

	ggprotoio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

// Handshake1 exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func Handshake1(ctx context.Context, c Conn) error {
	rpeer := c.RemotePeer()
	lpeer := c.LocalPeer()

	// setup up protobuf io
	maxSize := 4096
	r := ggprotoio.NewDelimitedReader(c, maxSize)
	w := ggprotoio.NewDelimitedWriter(c)
	localH := handshake.Handshake1Msg()
	remoteH := new(hspb.Handshake1)

	// send the outgoing handshake message
	if err := w.WriteMsg(localH); err != nil {
		return err
	}
	log.Debugf("%p sent my version (%s) to %s", c, localH, rpeer)
	log.Event(ctx, "handshake1Sent", lpeer)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := r.ReadMsg(remoteH); err != nil {
		return fmt.Errorf("could not receive remote version: %q", err)
	}
	log.Debugf("%p received remote version (%s) from %s", c, remoteH, rpeer)
	log.Event(ctx, "handshake1Received", lpeer)

	if err := handshake.Handshake1Compatible(localH, remoteH); err != nil {
		log.Infof("%s (%s) incompatible version with %s (%s)", lpeer, localH, rpeer, remoteH)
		return err
	}

	log.Debugf("%s version handshake compatible %s", lpeer, rpeer)
	return nil
}

// Handshake3 exchanges local and remote service information
func Handshake3(ctx context.Context, stream io.ReadWriter, c Conn) (*handshake.Handshake3Result, error) {
	rpeer := c.RemotePeer()
	lpeer := c.LocalPeer()

	// setup up protobuf io
	maxSize := 4096
	r := ggprotoio.NewDelimitedReader(stream, maxSize)
	w := ggprotoio.NewDelimitedWriter(stream)
	localH := handshake.Handshake3Msg(lpeer, c.RemoteMultiaddr())
	remoteH := new(hspb.Handshake3)

	// setup + send the message to remote
	if err := w.WriteMsg(localH); err != nil {
		return nil, err
	}
	log.Debugf("Handshake3: sent to %s", rpeer)
	log.Event(ctx, "handshake3Sent", lpeer, rpeer)

	// wait + listen for response
	if err := r.ReadMsg(remoteH); err != nil {
		return nil, fmt.Errorf("Handshake3 could not receive remote msg: %q", err)
	}
	log.Debugf("Handshake3: received from %s", rpeer)
	log.Event(ctx, "handshake3Received", lpeer, rpeer)

	// actually update our state based on the new knowledge
	res, err := handshake.Handshake3Update(lpeer, rpeer, remoteH)
	if err != nil {
		log.Errorf("Handshake3 failed to update %s", rpeer)
	}
	res.RemoteObservedAddress = c.RemoteMultiaddr()
	return res, nil
}
