// Package identify handles how peers identify with eachother upon
// connection to the network
package identify

import (
	"bytes"
	"errors"
	"strings"

	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	ci "github.com/jbenet/go-ipfs/crypto"
	peer "github.com/jbenet/go-ipfs/peer"
	u "github.com/jbenet/go-ipfs/util"
)

// List of supported protocols--each section in order of preference.
// Takes the form:  ECDH curves : Ciphers : Hashes
var SupportedExchanges = "P-256,P-224,P-384,P-521"
var SupportedCiphers = "AES-256,AES-128"
var SupportedHashes = "SHA256,SHA512,SHA1"

// ErrUnsupportedKeyType is returned when a private key cast/type switch fails.
var ErrUnsupportedKeyType = errors.New("unsupported key type")

// Performs initial communication with this peer to share node ID's and
// initiate communication.  (secureIn, secureOut, error)
func Handshake(ctx context.Context, self, remote *peer.Peer, in <-chan []byte, out chan<- []byte) (<-chan []byte, chan<- []byte, error) {

	encodedHello, err := encodedProtoHello(self)
	if err != nil {
		return nil, nil, err
	}

	// TODO(brian): select on |ctx|
	out <- encodedHello

	// Parse their Hello packet and generate an Exchange packet.
	// Exchange = (EphemeralPubKey, Signature)
	resp := <-in

	helloResp := new(Hello)
	err = proto.Unmarshal(resp, helloResp)
	if err != nil {
		return nil, nil, err
	}

	remote.PubKey, err = ci.UnmarshalPublicKey(helloResp.GetPubkey())
	if err != nil {
		return nil, nil, err
	}

	remote.ID, err = IDFromPubKey(remote.PubKey)
	if err != nil {
		return nil, nil, err
	}

	exchange, err := selectBest(SupportedExchanges, helloResp.GetExchanges())
	if err != nil {
		return nil, nil, err
	}

	cipherType, err := selectBest(SupportedCiphers, helloResp.GetCiphers())
	if err != nil {
		return nil, nil, err
	}

	hashType, err := selectBest(SupportedHashes, helloResp.GetHashes())
	if err != nil {
		return nil, nil, err
	}

	epubkey, done, err := ci.GenerateEKeyPair(exchange) // Generate EphemeralPubKey
	if err != nil {
		return nil, nil, err
	}

	var handshake bytes.Buffer // Gather corpus to sign.
	handshake.Write(encodedHello)
	handshake.Write(resp)
	handshake.Write(epubkey)

	exPacket := new(Exchange)

	exPacket.Epubkey = epubkey
	exPacket.Signature, err = self.PrivKey.Sign(handshake.Bytes())
	if err != nil {
		return nil, nil, err
	}

	exEncoded, err := proto.Marshal(exPacket)
	if err != nil {
		return nil, nil, err
	}

	out <- exEncoded

	// Parse their Exchange packet and generate a Finish packet.
	// Finish = E('Finish')
	resp1 := <-in

	exchangeResp := new(Exchange)
	err = proto.Unmarshal(resp1, exchangeResp)
	if err != nil {
		return nil, nil, err
	}

	var theirHandshake bytes.Buffer
	_, err = theirHandshake.Write(resp)
	if err != nil {
		return nil, nil, err
	}
	_, err = theirHandshake.Write(encodedHello)
	if err != nil {
		return nil, nil, err
	}
	_, err = theirHandshake.Write(exchangeResp.GetEpubkey())
	if err != nil {
		return nil, nil, err
	}

	ok, err := remote.PubKey.Verify(theirHandshake.Bytes(), exchangeResp.GetSignature())
	if err != nil {
		return nil, nil, err
	}

	if !ok {
		return nil, nil, errors.New("Bad signature!")
	}

	secret, err := done(exchangeResp.GetEpubkey())
	if err != nil {
		return nil, nil, err
	}

	myPubKey, err := self.PubKey.Bytes()
	if err != nil {
		return nil, nil, err
	}

	cmp := bytes.Compare(myPubKey, helloResp.GetPubkey())
	mIV, tIV, mCKey, tCKey, mMKey, tMKey := ci.KeyStretcher(cmp, cipherType, hashType, secret)

	secureIn := make(chan []byte)
	secureOut := make(chan []byte)

	go secureInProxy(in, secureIn, hashType, tIV, tCKey, tMKey)
	go secureOutProxy(out, secureOut, hashType, mIV, mCKey, mMKey)

	finished := []byte("Finished")

	secureOut <- finished
	resp2 := <-secureIn

	if bytes.Compare(resp2, finished) != 0 {
		return nil, nil, errors.New("Negotiation failed.")
	}

	u.DOut("[%s] identify: Got node id: %s\n", self.ID.Pretty(), remote.ID.Pretty())

	return secureIn, secureOut, nil
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

func secureInProxy(in <-chan []byte, secureIn chan<- []byte, hashType string, tIV, tCKey, tMKey []byte) {
	theirBlock, _ := aes.NewCipher(tCKey)
	theirCipher := cipher.NewCTR(theirBlock, tIV)

	theirMac, macSize := makeMac(hashType, tMKey)

	for {
		data, ok := <-in
		if !ok {
			close(secureIn)
			return
		}

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
			secureIn <- buff
		} else {
			secureIn <- nil
		}
	}
}

func secureOutProxy(out chan<- []byte, secureOut <-chan []byte, hashType string, mIV, mCKey, mMKey []byte) {
	myBlock, _ := aes.NewCipher(mCKey)
	myCipher := cipher.NewCTR(myBlock, mIV)

	myMac, macSize := makeMac(hashType, mMKey)

	for {
		data, ok := <-secureOut
		if !ok {
			close(out)
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

		out <- buff
	}
}

// IDFromPubKey returns Nodes ID given its public key
func IDFromPubKey(pk ci.PubKey) (peer.ID, error) {
	b, err := pk.Bytes()
	if err != nil {
		return nil, err
	}
	hash, err := u.Hash(b)
	if err != nil {
		return nil, err
	}
	return peer.ID(hash), nil
}

// Determines which algorithm to use.  Note:  f(a, b) = f(b, a)
func selectBest(myPrefs, theirPrefs string) (string, error) {
	// Person with greatest hash gets first choice.
	myHash, err := u.Hash([]byte(myPrefs))
	if err != nil {
		return "", err
	}

	theirHash, err := u.Hash([]byte(theirPrefs))
	if err != nil {
		return "", err
	}

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

func genRandHello(self *peer.Peer) (*Hello, error) {
	hello := new(Hello)

	// Generate and send Hello packet.
	// Hello = (rand, PublicKey, Supported)
	nonce := make([]byte, 16)
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}

	myPubKey, err := self.PubKey.Bytes()
	if err != nil {
		return nil, err
	}

	hello.Rand = nonce
	hello.Pubkey = myPubKey
	hello.Exchanges = &SupportedExchanges
	hello.Ciphers = &SupportedCiphers
	hello.Hashes = &SupportedHashes
	return hello, nil
}

func encodedProtoHello(self *peer.Peer) ([]byte, error) {
	h, err := genRandHello(self)
	if err != nil {
		return nil, err
	}

	return proto.Marshal(h)
}
