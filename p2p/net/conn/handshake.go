package conn

import (
	"fmt"

	handshake "github.com/jbenet/go-ipfs/p2p/net/handshake"
	hspb "github.com/jbenet/go-ipfs/p2p/net/handshake/pb"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ggprotoio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/gogoprotobuf/io"
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
