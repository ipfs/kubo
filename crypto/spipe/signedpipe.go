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
	"github.com/jbenet/go-ipfs/pipes"
)

type SignedPipe struct {
	pipes.Duplex
	insecure pipes.Duplex

	local  peer.Peer
	remote peer.Peer
	peers  peer.Peerstore

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSignedPipe(parctx context.Context, bufsize int, local peer.Peer,
	peers peer.Peerstore, insecure pipes.Duplex) (*SignedPipe, error) {

	ctx, cancel := context.WithCancel(parctx)

	sp := &SignedPipe{
		Duplex: pipes.Duplex{
			In:  make(chan []byte, bufsize),
			Out: make(chan []byte, bufsize),
		},
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

func (sp *SignedPipe) handshake() error {
	// Send them our public key
	pubk := sp.local.PubKey()
	pkb, err := pubk.Bytes()
	if err != nil {
		return err
	}

	sp.insecure.Out <- pkb
	theirPkb := <-sp.insecure.In

	theirPubKey, err := ci.UnmarshalPublicKey(theirPkb)
	if err != nil {
		return err
	}

	challenge := make([]byte, 32)
	rand.Read(challenge)

	enc, err := theirPubKey.Encrypt(challenge)
	if err != nil {
		return err
	}

	sp.insecure.Out <- enc
	theirEnc := <-sp.insecure.In

	unenc, err := sp.local.PrivKey().Unencrypt(theirEnc)
	if err != nil {
		return err
	}

	sig, err := sp.local.PrivKey().Sign(unenc)
	if err != nil {
		return err
	}

	sp.insecure.Out <- unenc
	theirUnenc := <-sp.insecure.In
	sp.insecure.Out <- sig
	theirSig := <-sp.insecure.In

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

	go sp.handleIn(theirPubKey)
	go sp.handleOut()

	finished := []byte("finished")
	sp.Out <- finished
	resp := <-sp.In
	if !bytes.Equal(resp, finished) {
		return errors.New("Handshake failed!")
	}

	return nil
}

func (sp *SignedPipe) handleOut() {
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

		sig, err := sp.local.PrivKey().Sign(data)
		if err != nil {
			log.Error("Error signing outgoing data: %s", err)
			continue
		}

		sdata.Data = data
		sdata.Sig = sig
		b, err := proto.Marshal(sdata)
		if err != nil {
			log.Error("Error marshaling signed data object: %s", err)
			continue
		}

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
		correct, err := theirPubkey.Verify(sdata.GetData(), sdata.GetSig())
		if err != nil {
			log.Error(err)
			continue
		}
		if !correct {
			log.Error("Received data with invalid signature!")
			continue
		}

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
