package secio

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"

	ci "github.com/jbenet/go-ipfs/crypto"
	pb "github.com/jbenet/go-ipfs/crypto/spipe/internal/pb"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
	eventlog "github.com/jbenet/go-ipfs/util/eventlog"
)

var log = eventlog.Logger("secio")

// ErrUnsupportedKeyType is returned when a private key cast/type switch fails.
var ErrUnsupportedKeyType = errors.New("unsupported key type")

// ErrClosed signals the closing of a connection.
var ErrClosed = errors.New("connection closed")

// nonceSize is the size of our nonces (in bytes)
const nonceSize = 16

// secureSession encapsulates all the parameters needed for encrypting
// and decrypting traffic from an insecure channel.
type secureSession struct {
	secure msgio.ReadWriteCloser

	insecure  io.ReadWriter
	insecureM msgio.ReadWriter

	localKey   ci.PrivKey
	localPeer  peer.ID
	remotePeer peer.ID

	local  encParams
	remote encParams

	sharedSecret []byte
}

func newSecureSession(local peer.ID, key ci.PrivKey) *secureSession {
	return &secureSession{localPeer: local, localKey: key}
}

// handsahke performs initial communication over insecure channel to share
// keys, IDs, and initiate communication, assigning all necessary params.
// requires the duplex channel to be a msgio.ReadWriter (for framed messaging)
func (s *secureSession) handshake(ctx context.Context, insecure io.ReadWriter) error {

	s.insecure = insecure
	s.insecureM = msgio.NewReadWriter(insecure)

	// =============================================================================
	// step 1. Propose -- propose cipher suite + send pubkeys + nonce

	// Generate and send Hello packet.
	// Hello = (rand, PublicKey, Supported)
	nonceOut := make([]byte, nonceSize)
	_, err := rand.Read(nonceOut)
	if err != nil {
		return err
	}

	log.Debugf("handshake: %s <--start--> %s", s.localPeer, s.remotePeer)
	log.Event(ctx, "secureHandshakeStart", s.localPeer)
	s.local.permanentPubKey = s.localKey.GetPublic()
	myPubKeyBytes, err := s.local.permanentPubKey.Bytes()
	if err != nil {
		return err
	}

	proposeOut := new(pb.Propose)
	proposeOut.Rand = nonceOut
	proposeOut.Pubkey = myPubKeyBytes
	proposeOut.Exchanges = &SupportedExchanges
	proposeOut.Ciphers = &SupportedCiphers
	proposeOut.Hashes = &SupportedHashes

	// Send Propose packet (respects ctx)
	proposeOutBytes, err := writeMsgCtx(ctx, s.insecureM, proposeOut)
	if err != nil {
		return err
	}

	// Receive + Parse their Propose packet and generate an Exchange packet.
	proposeIn := new(pb.Propose)
	proposeInBytes, err := readMsgCtx(ctx, s.insecureM, proposeIn)
	if err != nil {
		return err
	}

	// =============================================================================
	// step 1.1 Identify -- get identity from their key

	// get remote identity
	s.remote.permanentPubKey, err = ci.UnmarshalPublicKey(proposeIn.GetPubkey())
	if err != nil {
		return err
	}

	// get peer id
	s.remotePeer, err = peer.IDFromPublicKey(s.remote.permanentPubKey)
	if err != nil {
		return err
	}
	// log.Debugf("%s Remote Peer Identified as %s", s.localPeer, s.remotePeer)

	// =============================================================================
	// step 1.2 Selection -- select/agree on best encryption parameters

	// to determine order, use cmp(H(lr||rpk), H(rr||lpk)).
	oh1 := u.Hash(append(proposeIn.GetPubkey(), nonceOut...))
	oh2 := u.Hash(append(myPubKeyBytes, proposeIn.GetRand()...))
	order := bytes.Compare(oh1, oh2)
	s.local.curveT, err = selectBest(order, SupportedExchanges, proposeIn.GetExchanges())
	if err != nil {
		return err
	}

	s.local.cipherT, err = selectBest(order, SupportedCiphers, proposeIn.GetCiphers())
	if err != nil {
		return err
	}

	s.local.hashT, err = selectBest(order, SupportedHashes, proposeIn.GetHashes())
	if err != nil {
		return err
	}

	// we use the same params for both directions (must choose same curve)
	// WARNING: if they dont SelectBest the same way, this won't work...
	s.remote.curveT = s.local.curveT
	s.remote.cipherT = s.local.cipherT
	s.remote.hashT = s.local.hashT

	// =============================================================================
	// step 2. Exchange -- exchange (signed) ephemeral keys. verify signatures.

	// Generate EphemeralPubKey
	var genSharedKey ci.GenSharedKey
	s.local.ephemeralPubKey, genSharedKey, err = ci.GenerateEKeyPair(s.local.curveT)

	// Gather corpus to sign.
	var selectionOut bytes.Buffer
	selectionOut.Write(proposeOutBytes)
	selectionOut.Write(proposeInBytes)
	selectionOut.Write(s.local.ephemeralPubKey)
	selectionOutBytes := selectionOut.Bytes()

	exchangeOut := new(pb.Exchange)
	exchangeOut.Epubkey = s.local.ephemeralPubKey
	exchangeOut.Signature, err = s.localKey.Sign(selectionOutBytes)
	if err != nil {
		return err
	}

	// Send Propose packet (respects ctx)
	if _, err := writeMsgCtx(ctx, s.insecureM, exchangeOut); err != nil {
		return err
	}

	// Receive + Parse their Propose packet and generate an Exchange packet.
	exchangeIn := new(pb.Exchange)
	if _, err := readMsgCtx(ctx, s.insecureM, exchangeIn); err != nil {
		return err
	}

	// =============================================================================
	// step 2.1. Verify -- verify their exchange packet is good.

	// get their ephemeral pub key
	s.remote.ephemeralPubKey = exchangeIn.GetEpubkey()

	var selectionIn bytes.Buffer
	selectionIn.Write(proposeInBytes)
	selectionIn.Write(proposeOutBytes)
	selectionIn.Write(s.remote.ephemeralPubKey)
	selectionInBytes := selectionIn.Bytes()

	// u.POut("Remote Peer Identified as %s\n", s.remote)
	sigOK, err := s.local.permanentPubKey.Verify(selectionInBytes, exchangeIn.GetSignature())
	if err != nil {
		return err
	}

	if !sigOK {
		return errors.New("Bad signature!")
	}

	// =============================================================================
	// step 2.2. Keys -- generate keys for mac + encryption

	// OK! seems like we're good to go.
	s.sharedSecret, err = genSharedKey(exchangeIn.GetEpubkey())
	if err != nil {
		return err
	}

	// generate two sets of keys (stretching)
	k1, k2 := ci.KeyStretcher(s.local.cipherT, s.local.hashT, s.sharedSecret)

	// use random nonces to decide order.
	switch order {
	case 1:
	case -1:
		k1, k2 = k2, k1 // swap
	default:
		log.Error("WOAH: same keys (AND same nonce: 1/(2^128) chance!).")
		// this shouldn't happen. must determine order another way.
		// use the same keys but, make sure to copy underlying data!
		copy(k2.IV, k1.IV)
		copy(k2.MacKey, k1.MacKey)
		copy(k2.CipherKey, k1.CipherKey)
	}
	s.local.keys = k1
	s.remote.keys = k2

	// =============================================================================
	// step 2.3. MAC + Cipher -- prepare MAC + cipher

	if err := s.local.makeMacAndCipher(); err != nil {
		return err
	}

	if err := s.remote.makeMacAndCipher(); err != nil {
		return err
	}

	// =============================================================================
	// step 3. Finish -- send expected message (the nonces), verify encryption works

	// setup ETM ReadWriter
	w := NewETMWriter(s.insecure, s.local.cipher, s.local.mac)
	r := NewETMReader(s.insecure, s.remote.cipher, s.remote.mac)
	s.secure = msgio.Combine(w, r).(msgio.ReadWriteCloser)

	// send their Nonce.
	if _, err := s.secure.Write(proposeIn.GetRand()); err != nil {
		return fmt.Errorf("Failed to write Finish nonce: %s", err)
	}

	// read our Nonce
	nonceOut2 := make([]byte, len(nonceOut))
	if _, err := io.ReadFull(s.secure, nonceOut2); err != nil {
		return fmt.Errorf("Failed to read Finish nonce: %s", err)
	}
	if !bytes.Equal(nonceOut, nonceOut2) {
		return fmt.Errorf("Failed to read our encrypted nonce: %s != %s", nonceOut2, nonceOut)
	}

	// Whew! ok, that's all folks.
	log.Debugf("handshake: %s <--finish--> %s", s.localPeer, s.remotePeer)
	log.Event(ctx, "secureHandshakeFinish", s.localPeer, s.remotePeer)
	return nil
}
