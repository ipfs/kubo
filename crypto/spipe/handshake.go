// Package spipe handles establishing secure communication between two peers.

package spipe

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	bfish "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.crypto/blowfish"
	"hash"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	ci "github.com/jbenet/go-ipfs/crypto"
	pb "github.com/jbenet/go-ipfs/crypto/spipe/internal/pb"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("handshake")

// List of supported ECDH curves
var SupportedExchanges = "P-256,P-224,P-384,P-521"

// List of supported Ciphers
var SupportedCiphers = "AES-256,AES-128,Blowfish"

// List of supported Hashes
var SupportedHashes = "SHA256,SHA512,SHA1"

// ErrUnsupportedKeyType is returned when a private key cast/type switch fails.
var ErrUnsupportedKeyType = errors.New("unsupported key type")

// ErrClosed signals the closing of a connection.
var ErrClosed = errors.New("connection closed")

// handsahke performs initial communication over insecure channel to share
// keys, IDs, and initiate communication.
func (s *SecurePipe) handshake() error {
	// Generate and send Hello packet.
	// Hello = (rand, PublicKey, Supported)
	nonce := make([]byte, 16)
	_, err := rand.Read(nonce)
	if err != nil {
		return err
	}

	log.Debugf("handshake: %s <--> %s", s.local, s.remote)
	myPubKey, err := s.local.PubKey().Bytes()
	if err != nil {
		return err
	}

	proposeMsg := new(pb.Propose)
	proposeMsg.Rand = nonce
	proposeMsg.Pubkey = myPubKey
	proposeMsg.Exchanges = &SupportedExchanges
	proposeMsg.Ciphers = &SupportedCiphers
	proposeMsg.Hashes = &SupportedHashes

	encoded, err := proto.Marshal(proposeMsg)
	if err != nil {
		return err
	}

	// Send our Propose packet
	select {
	case s.insecure.Out <- encoded:
	case <-s.ctx.Done():
		return ErrClosed
	}

	// Parse their Propose packet and generate an Exchange packet.
	// Exchange = (EphemeralPubKey, Signature)
	var resp []byte
	select {
	case <-s.ctx.Done():
		return ErrClosed
	case resp = <-s.insecure.In:
	}

	// u.POut("received encoded handshake\n")
	proposeResp := new(pb.Propose)
	err = proto.Unmarshal(resp, proposeResp)
	if err != nil {
		return err
	}

	// get remote identity
	remotePubKey, err := ci.UnmarshalPublicKey(proposeResp.GetPubkey())
	if err != nil {
		return err
	}

	// get or construct peer
	s.remote, err = getOrConstructPeer(s.peers, remotePubKey)
	if err != nil {
		return err
	}
	log.Debugf("%s Remote Peer Identified as %s", s.local, s.remote)

	exchange, err := SelectBest(SupportedExchanges, proposeResp.GetExchanges())
	if err != nil {
		return err
	}

	cipherType, err := SelectBest(SupportedCiphers, proposeResp.GetCiphers())
	if err != nil {
		return err
	}

	hashType, err := SelectBest(SupportedHashes, proposeResp.GetHashes())
	if err != nil {
		return err
	}

	// u.POut("Selected %s %s %s\n", exchange, cipherType, hashType)
	epubkey, genSharedKey, err := ci.GenerateEKeyPair(exchange) // Generate EphemeralPubKey

	var handshake bytes.Buffer // Gather corpus to sign.
	handshake.Write(encoded)
	handshake.Write(resp)
	handshake.Write(epubkey)

	exPacket := new(pb.Exchange)

	exPacket.Epubkey = epubkey
	exPacket.Signature, err = s.local.PrivKey().Sign(handshake.Bytes())
	if err != nil {
		return err
	}

	exEncoded, err := proto.Marshal(exPacket)

	// send out Exchange packet
	select {
	case s.insecure.Out <- exEncoded:
	case <-s.ctx.Done():
		return ErrClosed
	}

	// Parse their Exchange packet and generate a Finish packet.
	// Finish = E('Finish')
	var resp1 []byte
	select {
	case <-s.ctx.Done():
		return ErrClosed
	case resp1 = <-s.insecure.In:
	}

	exchangeResp := new(pb.Exchange)
	err = proto.Unmarshal(resp1, exchangeResp)
	if err != nil {
		return err
	}

	var theirHandshake bytes.Buffer
	theirHandshake.Write(resp)
	theirHandshake.Write(encoded)
	theirHandshake.Write(exchangeResp.GetEpubkey())

	// u.POut("Remote Peer Identified as %s\n", s.remote)
	ok, err := s.remote.PubKey().Verify(theirHandshake.Bytes(), exchangeResp.GetSignature())
	if err != nil {
		return err
	}

	if !ok {
		return errors.New("Bad signature!")
	}

	secret, err := genSharedKey(exchangeResp.GetEpubkey())
	if err != nil {
		return err
	}

	cmp := bytes.Compare(myPubKey, proposeResp.GetPubkey())
	//mIV, tIV, mCKey, tCKey, mMKey, tMKey := ci.KeyStretcher(cmp, cipherType, hashType, secret)
	ci.KeyStretcher(cmp, cipherType, hashType, secret)

	//go s.handleSecureIn(hashType, cipherType, tIV, tCKey, tMKey)
	//go s.handleSecureOut(hashType, cipherType, mIV, mCKey, mMKey)

	// Disable Secure Channel
	go func(sp *SecurePipe) {
		for {
			select {
			case <-sp.ctx.Done():
				return
			case m, ok := <-sp.insecure.In:
				if !ok {
					sp.cancel()
					return
				}
				sp.In <- m
			}
		}
	}(s)
	go func(sp *SecurePipe) {
		for {
			select {
			case <-sp.ctx.Done():
				return
			case m, ok := <-sp.Out:
				if !ok {
					sp.cancel()
					return
				}
				sp.insecure.Out <- m
			}
		}
	}(s)

	finished := []byte("Finished")

	// send finished msg
	select {
	case <-s.ctx.Done():
		return ErrClosed
	case s.Out <- finished:
	}

	// recv finished msg
	var resp2 []byte
	select {
	case <-s.ctx.Done():
		return ErrClosed
	case resp2 = <-s.In:
	}

	if bytes.Compare(resp2, finished) != 0 {
		return fmt.Errorf("Negotiation failed, got: %s", resp2)
	}

	log.Debugf("%s handshake: Got node id: %s", s.local, s.remote)
	return nil
}

func makeMac(hashType string, key []byte) (hash.Hash, int) {
	switch hashType {
	case "SHA1":
		return hmac.New(sha1.New, key), sha1.Size
	case "SHA512":
		return hmac.New(sha512.New, key), sha512.Size
	default:
		return hmac.New(sha256.New, key), sha256.Size
	}
}

func makeCipher(cipherType string, CKey []byte) (cipher.Block, error) {
	switch cipherType {
	case "AES-128", "AES-256":
		return aes.NewCipher(CKey)
	case "Blowfish":
		return bfish.NewCipher(CKey)
	default:
		return nil, fmt.Errorf("Unrecognized cipher string: %s", cipherType)
	}
}

func (s *SecurePipe) handleSecureIn(hashType, cipherType string, tIV, tCKey, tMKey []byte) {
	theirBlock, err := makeCipher(cipherType, tCKey)
	if err != nil {
		log.Criticalf("Invalid Cipher: %s", err)
		s.cancel()
		return
	}
	theirCipher := cipher.NewCTR(theirBlock, tIV)

	theirMac, macSize := makeMac(hashType, tMKey)

	for {
		var data []byte
		ok := true

		select {
		case <-s.ctx.Done():
			ok = false // return out
		case data, ok = <-s.insecure.In:
		}

		if !ok {
			close(s.Duplex.In)
			return
		}

		// log.Debug("[peer %s] secure in [from = %s] %d", s.local, s.remote, len(data))
		if len(data) <= macSize {
			continue
		}

		mark := len(data) - macSize
		buff := make([]byte, mark)

		theirCipher.XORKeyStream(buff, data[0:mark])

		theirMac.Write(data[0:mark])
		expected := theirMac.Sum(nil)
		theirMac.Reset()

		hmacOk := hmac.Equal(data[mark:], expected)

		if hmacOk {
			s.Duplex.In <- buff
		} else {
			s.Duplex.In <- nil
		}
	}
}

func (s *SecurePipe) handleSecureOut(hashType, cipherType string, mIV, mCKey, mMKey []byte) {
	myBlock, err := makeCipher(cipherType, mCKey)
	if err != nil {
		log.Criticalf("Invalid Cipher: %s", err)
		s.cancel()
		return
	}
	myCipher := cipher.NewCTR(myBlock, mIV)

	myMac, macSize := makeMac(hashType, mMKey)

	for {
		var data []byte
		ok := true

		select {
		case <-s.ctx.Done():
			ok = false // return out
		case data, ok = <-s.Out:
		}

		if !ok {
			close(s.insecure.Out)
			return
		}

		if len(data) == 0 {
			continue
		}

		buff := make([]byte, len(data)+macSize)

		myCipher.XORKeyStream(buff, data)

		myMac.Write(buff[0:len(data)])
		copy(buff[len(data):], myMac.Sum(nil))
		myMac.Reset()

		// log.Debug("[peer %s] secure out [to = %s] %d", s.local, s.remote, len(buff))
		s.insecure.Out <- buff
	}
}

// Determines which algorithm to use.  Note:  f(a, b) = f(b, a)
func SelectBest(myPrefs, theirPrefs string) (string, error) {
	// Person with greatest hash gets first choice.
	myHash := u.Hash([]byte(myPrefs))
	theirHash := u.Hash([]byte(theirPrefs))

	cmp := bytes.Compare(myHash, theirHash)
	var firstChoiceArr, secChoiceArr []string

	if cmp == -1 {
		firstChoiceArr = strings.Split(theirPrefs, ",")
		secChoiceArr = strings.Split(myPrefs, ",")
	} else if cmp == 1 {
		firstChoiceArr = strings.Split(myPrefs, ",")
		secChoiceArr = strings.Split(theirPrefs, ",")
	} else { // Exact same preferences.
		myPrefsArr := strings.Split(myPrefs, ",")
		return myPrefsArr[0], nil
	}

	for _, secChoice := range secChoiceArr {
		for _, firstChoice := range firstChoiceArr {
			if firstChoice == secChoice {
				return firstChoice, nil
			}
		}
	}

	return "", errors.New("No algorithms in common!")
}

// getOrConstructPeer attempts to fetch a peer from a peerstore.
// if succeeds, verify ID and PubKey match.
// else, construct it.
func getOrConstructPeer(peers peer.Peerstore, rpk ci.PubKey) (peer.Peer, error) {

	rid, err := peer.IDFromPubKey(rpk)
	if err != nil {
		return nil, err
	}

	npeer, err := peers.Get(rid)
	if err != nil {
		return nil, err // unexpected error happened.
	}

	// public key verification happens in Peer.VerifyAndSetPubKey
	if err := npeer.VerifyAndSetPubKey(rpk); err != nil {
		return nil, err // pubkey mismatch or other problem
	}
	return npeer, nil
}
