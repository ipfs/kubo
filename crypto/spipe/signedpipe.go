package spipe

import (
	"bytes"
	"crypto/rand"
	"errors"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	ci "github.com/jbenet/go-ipfs/crypto"
	pb "github.com/jbenet/go-ipfs/crypto/spipe/internal/pb"
	"github.com/jbenet/go-ipfs/peer"
	"github.com/jbenet/go-ipfs/util/pipes"
)

type SignedPipe struct {
	pipes.Duplex
	insecure pipes.Duplex

	local  peer.Peer
	remote peer.Peer
	peers  peer.Peerstore

	ctx    context.Context
	cancel context.CancelFunc

	localMsgID  uint64
	removeMsgID uint64
}

// secureChallengeSize is a constant that determines the initial challenge, and every subsequent
// sequence number. It should be large enough to be unguessable by adversaries (128+ bits).
// (SECURITY WARNING)
const secureChallengeSize = (256 / 32)

func NewSignedPipe(parctx context.Context, bufsize int, local peer.Peer,
	peers peer.Peerstore, insecure pipes.Duplex) (*SignedPipe, error) {

	ctx, cancel := context.WithCancel(parctx)

	sp := &SignedPipe{
		Duplex:   pipes.NewDuplex(bufsize),
		local:    local,
		peers:    peers,
		insecure: insecure,

		ctx:    ctx,
		cancel: cancel,
	}

	if err := sp.handshake(); err != nil {
		sp.Close()
		return nil, err
	}
	return sp, nil
}

func (sp *SignedPipe) trySend(b []byte) bool {
	select {
	case <-sp.ctx.Done():
		return false
	case sp.insecure.Out <- b:
		return true
	}
}

func (sp *SignedPipe) tryRecv() ([]byte, bool) {
	select {
	case <-sp.ctx.Done():
		return nil, false
	case data, ok := <-sp.insecure.In:
		if !ok {
			return nil, false
		}
		return data, true
	}
}

// reduceChallenge reduces a series of bytes into a
// single uint64 we can use as a seed for message IDs
func reduceChallenge(cha []byte) uint64 {
	var out uint64
	for _, b := range cha {
		out ^= uint64(b)
		out = out << 1
	}
	return out
}

func (sp *SignedPipe) handshake() error {
	// Send them our public key
	pubk := sp.local.PubKey()
	pkb, err := pubk.Bytes()
	if err != nil {
		return err
	}

	// Exchange public keys with remote peer
	if !sp.trySend(pkb) {
		return context.Canceled
	}
	theirPkb := <-sp.insecure.In

	theirPubKey, err := ci.UnmarshalPublicKey(theirPkb)
	if err != nil {
		return err
	}

	challenge := make([]byte, secureChallengeSize)
	rand.Read(challenge)

	enc, err := theirPubKey.Encrypt(challenge)
	if err != nil {
		return err
	}

	chsig, err := sp.local.PrivKey().Sign(challenge)
	if err != nil {
		return err
	}

	if !sp.trySend(enc) {
		return context.Canceled
	}
	if !sp.trySend(chsig) {
		return context.Canceled
	}

	theirEnc, ok := sp.tryRecv()
	if !ok {
		return context.Canceled
	}
	theirChSig, ok := sp.tryRecv()
	if !ok {
		return context.Canceled
	}

	// Decrypt and verify their challenge
	unenc, err := sp.local.PrivKey().Decrypt(theirEnc)
	if err != nil {
		return err
	}
	ok, err = theirPubKey.Verify(unenc, theirChSig)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("Invalid signature!")
	}

	// Sign the unencrypted challenge, and send it back
	sig, err := sp.local.PrivKey().Sign(unenc)
	if err != nil {
		return err
	}

	if !sp.trySend(unenc) {
		return context.Canceled
	}
	if !sp.trySend(sig) {
		return context.Canceled
	}
	theirUnenc, ok := sp.tryRecv()
	if !ok {
		return context.Canceled
	}
	theirSig, ok := sp.tryRecv()
	if !ok {
		return context.Canceled
	}

	// Verify that they correctly unecrypted the challenge
	if !bytes.Equal(theirUnenc, challenge) {
		return errors.New("received bad challenge response")
	}

	correct, err := theirPubKey.Verify(theirUnenc, theirSig)
	if err != nil {
		return err
	}

	if !correct {
		return errors.New("Incorrect signature on challenge")
	}

	sp.removeMsgID = reduceChallenge(challenge)
	sp.localMsgID = reduceChallenge(unenc)

	go sp.handleIn(theirPubKey)
	go sp.handleOut(sp.local.PrivKey())

	finished := []byte("finished")

	select {
	case <-sp.ctx.Done():
		return context.Canceled
	case sp.Out <- finished:
	}

	var resp []byte
	select {
	case <-sp.ctx.Done():
		return context.Canceled
	case resp, ok = <-sp.In:
		if !ok {
			return errors.New("Channel closed before handshake finished.")
		}
	}
	if !bytes.Equal(resp, finished) {
		return errors.New("Handshake failed!")
	}

	return nil
}

func (sp *SignedPipe) handleOut(pk ci.PrivKey) {
	for {
		var data []byte
		var ok bool
		select {
		case <-sp.ctx.Done():
			return
		case data, ok = <-sp.Out:
			if !ok {
				log.Warning("pipe closed!")
				return
			}
		}

		sdata := new(pb.DataSig)

		sig, err := pk.Sign(data)
		if err != nil {
			log.Error("Error signing outgoing data: %s", err)
			return
		}

		sdata.Data = data
		sdata.Signature = sig
		sdata.Id = proto.Uint64(sp.localMsgID)
		b, err := proto.Marshal(sdata)
		if err != nil {
			log.Error("Error marshaling signed data object: %s", err)
			return
		}
		sp.localMsgID++

		select {
		case sp.insecure.Out <- b:
		case <-sp.ctx.Done():
			log.Debug("Context finished before send could occur")
			return
		}
	}
}

func (sp *SignedPipe) handleIn(theirPubkey ci.PubKey) {
	for {
		var data []byte
		var ok bool
		select {
		case <-sp.ctx.Done():
			return
		case data, ok = <-sp.insecure.In:
			if !ok {
				log.Debug("Signed pipe closed")
				return
			}
		}

		sdata := new(pb.DataSig)
		err := proto.Unmarshal(data, sdata)
		if err != nil {
			log.Error("Failed to unmarshal sigdata object")
			continue
		}
		correct, err := theirPubkey.Verify(sdata.GetData(), sdata.GetSignature())
		if err != nil {
			log.Error(err)
			continue
		}
		if !correct {
			log.Error("Received data with invalid signature!")
			continue
		}

		if sdata.GetId() != sp.removeMsgID {
			log.Critical("Out of order message id!")
			return
		}
		sp.removeMsgID++

		select {
		case <-sp.ctx.Done():
			return
		case sp.In <- sdata.GetData():
		}
	}
}

func (sp *SignedPipe) Close() error {
	sp.cancel()
	return nil
}
