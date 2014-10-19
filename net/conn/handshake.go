package conn

import (
	"errors"
	"fmt"

	handshake "github.com/jbenet/go-ipfs/net/handshake"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

// VersionHandshake exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func VersionHandshake(ctx context.Context, c Conn) error {
	rpeer := c.RemotePeer()
	lpeer := c.LocalPeer()

	var remoteH, localH *handshake.Handshake1
	localH = handshake.CurrentHandshake()

	myVerBytes, err := proto.Marshal(localH)
	if err != nil {
		return err
	}

	c.Out() <- myVerBytes
	log.Debug("Sent my version (%s) to %s", localH, rpeer)

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-c.Closed():
		return errors.New("remote closed connection during version exchange")

	case data, ok := <-c.In():
		if !ok {
			return fmt.Errorf("error retrieving from conn: %v", rpeer)
		}

		remoteH = new(handshake.Handshake1)
		err = proto.Unmarshal(data, remoteH)
		if err != nil {
			return fmt.Errorf("could not decode remote version: %q", err)
		}

		log.Debug("Received remote version (%s) from %s", remoteH, rpeer)
	}

	if err := handshake.Compatible(localH, remoteH); err != nil {
		log.Info("%s (%s) incompatible version with %s (%s)", lpeer, localH, rpeer, remoteH)
		return err
	}

	log.Debug("%s version handshake compatible %s", lpeer, rpeer)
	return nil
}
