package conn

import (
	"fmt"

	handshake "github.com/jbenet/go-ipfs/net/handshake"
	hspb "github.com/jbenet/go-ipfs/net/handshake/pb"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
)

// Handshake1 exchanges local and remote versions and compares them
// closes remote and returns an error in case of major difference
func Handshake1(ctx context.Context, c Conn) error {
	rpeer := c.RemotePeer()
	lpeer := c.LocalPeer()

	var remoteH, localH *hspb.Handshake1
	localH = handshake.Handshake1Msg()

	myVerBytes, err := proto.Marshal(localH)
	if err != nil {
		return err
	}

	if err := CtxWriteMsg(ctx, c, myVerBytes); err != nil {
		return err
	}
	log.Debugf("%p sent my version (%s) to %s", c, localH, rpeer)

	data, err := CtxReadMsg(ctx, c)
	if err != nil {
		return err
	}

	remoteH = new(hspb.Handshake1)
	err = proto.Unmarshal(data, remoteH)
	if err != nil {
		return fmt.Errorf("could not decode remote version: %q", err)
	}
	log.Debugf("%p received remote version (%s) from %s", c, remoteH, rpeer)

	if err := handshake.Handshake1Compatible(localH, remoteH); err != nil {
		log.Infof("%s (%s) incompatible version with %s (%s)", lpeer, localH, rpeer, remoteH)
		return err
	}

	log.Debugf("%s version handshake compatible %s", lpeer, rpeer)
	return nil
}

// Handshake3 exchanges local and remote service information
func Handshake3(ctx context.Context, c Conn) (*handshake.Handshake3Result, error) {
	rpeer := c.RemotePeer()
	lpeer := c.LocalPeer()

	// setup + send the message to remote
	var remoteH, localH *hspb.Handshake3
	localH = handshake.Handshake3Msg(lpeer, c.RemoteMultiaddr())
	localB, err := proto.Marshal(localH)
	if err != nil {
		return nil, err
	}

	if err := CtxWriteMsg(ctx, c, localB); err != nil {
		return nil, err
	}
	log.Debugf("Handshake1: sent to %s", rpeer)

	// wait + listen for response
	remoteB, err := CtxReadMsg(ctx, c)
	if err != nil {
		return nil, err
	}

	remoteH = new(hspb.Handshake3)
	err = proto.Unmarshal(remoteB, remoteH)
	if err != nil {
		return nil, fmt.Errorf("Handshake3 could not decode remote msg: %q", err)
	}

	log.Debugf("Handshake3 received from %s", rpeer)

	// actually update our state based on the new knowledge
	res, err := handshake.Handshake3Update(lpeer, rpeer, remoteH)
	if err != nil {
		log.Errorf("Handshake3 failed to update %s", rpeer)
	}
	res.RemoteObservedAddress = c.RemoteMultiaddr()
	return res, nil
}
