// The identify package handles how peers identify with eachother upon
// connection to the network
package identify

import (
	"bytes"
	"errors"

	"crypto/aes"
	"crypto/cipher"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"math/big"
	"strings"

	proto "code.google.com/p/goprotobuf/proto"
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

// Perform initial communication with this peer to share node ID's and
// initiate communication.  (secureIn, secureOut, error)
func Handshake(self, remote *peer.Peer, in, out chan []byte) (chan []byte, chan []byte, error) {
	// Generate and send Hello packet.
	// Hello = (rand, PublicKey, Supported)
	nonce := make([]byte, 16)
	rand.Read(nonce)

	hello := new(Hello)

	myPubKey, err := self.PubKey.Bytes()
	if err != nil {
		return nil, nil, err
	}

	hello.Rand = nonce
	hello.Pubkey = myPubKey
	hello.Exchanges = &SupportedExchanges
	hello.Ciphers = &SupportedCiphers
	hello.Hashes = &SupportedHashes

	encoded, err := proto.Marshal(hello)
	if err != nil {
		return nil, nil, err
	}

	out <- encoded

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

	remote.ID, err = IdFromPubKey(remote.PubKey)
	if err != nil {
		return nil, nil, err
	}

	exchange, err := selectBest(SupportedExchanges, helloResp.GetExchanges())
	if err != nil {
		return nil, nil, err
	}

	cipherType, err := selectBest(SupportedExchanges, helloResp.GetCiphers())
	if err != nil {
		return nil, nil, err
	}

	hashType, err := selectBest(SupportedExchanges, helloResp.GetHashes())
	if err != nil {
		return nil, nil, err
	}

	epubkey, done, err := generateEPubKey(exchange) // Generate EphemeralPubKey

	var handshake bytes.Buffer // Gather corpus to sign.
	handshake.Write(encoded)
	handshake.Write(resp)
	handshake.Write(epubkey)

	exPacket := new(Exchange)

	exPacket.Epubkey = epubkey
	exPacket.Signature, err = self.PrivKey.Sign(handshake.Bytes())
	if err != nil {
		return nil, nil, err
	}

	exEncoded, err := proto.Marshal(exPacket)

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
	theirHandshake.Write(resp)
	theirHandshake.Write(encoded)
	theirHandshake.Write(exchangeResp.GetEpubkey())

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

	cmp := bytes.Compare(myPubKey, helloResp.GetPubkey())
	mIV, tIV, mCKey, tCKey, mMKey, tMKey := keyGenerator(cmp, cipherType, hashType, secret)

	secureIn := make(chan []byte)
	secureOut := make(chan []byte)

	go func() {
		myBlock, _ := aes.NewCipher(mCKey)
		myCipher := cipher.NewCTR(myBlock, mIV)

		theirBlock, _ := aes.NewCipher(tCKey)
		theirCipher := cipher.NewCTR(theirBlock, tIV)

		var myMac, theirMac hash.Hash
		var macSize int

		switch hashType {
		case "SHA1":
			myMac = hmac.New(sha1.New, mMKey)
			theirMac = hmac.New(sha1.New, tMKey)
			macSize = 20

		case "SHA256":
			myMac = hmac.New(sha256.New, mMKey)
			theirMac = hmac.New(sha256.New, tMKey)
			macSize = 32

		case "SHA512":
			myMac = hmac.New(sha512.New, mMKey)
			theirMac = hmac.New(sha512.New, tMKey)
			macSize = 64
		}

		for {
			select {
			case data, ok := <-secureOut:
				if !ok {
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

			case data, ok := <-in:
				if !ok {
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
	}()

	u.DOut("[%s] identify: Got node id: %s\n", self.ID.Pretty(), remote.ID.Pretty())

	return secureIn, secureOut, nil
}

func IdFromPubKey(pk ci.PubKey) (peer.ID, error) {
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

// Generates a set of keys for each party by stretching the shared key.
// (myIV, theirIV, myCipherKey, theirCipherKey, myMACKey, theirMACKey)
func keyGenerator(cmp int, cipherType string, hashType string, secret []byte) ([]byte, []byte, []byte, []byte, []byte, []byte) {
	var cipherKeySize int
	switch cipherType {
	case "AES128":
		cipherKeySize = 2 * 16
	case "AES256":
		cipherKeySize = 2 * 32
	}

	ivSize := 16
	hmacKeySize := 20

	seed := []byte("key expansion")

	result := make([]byte, 2*(ivSize+cipherKeySize+hmacKeySize))

	var h func() hash.Hash

	switch hashType {
	case "SHA1":
		h = sha1.New
	case "SHA256":
		h = sha256.New
	case "SHA512":
		h = sha512.New
	}

	m := hmac.New(h, secret)
	m.Write(seed)

	a := m.Sum(nil)

	j := 0
	for j < len(result) {
		m.Reset()
		m.Write(a)
		m.Write(seed)
		b := m.Sum(nil)

		todo := len(b)

		if j+todo > len(result) {
			todo = len(result) - j
		}

		copy(result[j:j+todo], b)

		j += todo

		m.Reset()
		m.Write(a)
		a = m.Sum(nil)
	}

	myResult := make([]byte, ivSize+cipherKeySize+hmacKeySize)
	theirResult := make([]byte, ivSize+cipherKeySize+hmacKeySize)

	half := len(result) / 2

	if cmp == 1 {
		copy(myResult, result[:half])
		copy(theirResult, result[half:])
	} else if cmp == -1 {
		copy(myResult, result[half:])
		copy(theirResult, result[:half])
	} else { // Shouldn't happen, but oh well.
		copy(myResult, result[half:])
		copy(theirResult, result[half:])
	}

	myIV := myResult[0:ivSize]
	myCKey := myResult[ivSize : ivSize+cipherKeySize]
	myMKey := myResult[ivSize+cipherKeySize:]

	theirIV := theirResult[0:ivSize]
	theirCKey := theirResult[ivSize : ivSize+cipherKeySize]
	theirMKey := theirResult[ivSize+cipherKeySize:]

	return myIV, theirIV, myCKey, theirCKey, myMKey, theirMKey
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

// Generates an ephemeral public key and returns a function that will compute
// the shared secret key.
//
// Focuses only on ECDH now, but can be made more general in the future.
func generateEPubKey(exchange string) ([]byte, func([]byte) ([]byte, error), error) {
	genKeyPair := func(curve elliptic.Curve) ([]byte, []byte, error) {
		priv, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
		if err != nil {
			return nil, nil, err
		}

		var pubKey bytes.Buffer
		pubKey.Write(x.Bytes())
		pubKey.Write(y.Bytes())

		return pubKey.Bytes(), priv, nil
	}

	genSec := func(curve elliptic.Curve, theirPub []byte, myPriv []byte) ([]byte, error) {
		// Verify and unpack node's public key.
		curveSize := curve.Params().BitSize

		if len(theirPub) != (curveSize / 2) {
			return nil, errors.New("Malformed public key.")
		}

		bound := (curveSize / 8)
		x := big.NewInt(0)
		y := big.NewInt(0)

		x.SetBytes(theirPub[0:bound])
		y.SetBytes(theirPub[bound : bound*2])

		if !curve.IsOnCurve(x, y) {
			return nil, errors.New("Invalid public key.")
		}

		// Generate shared secret.
		secret, _ := curve.ScalarMult(x, y, myPriv)

		return secret.Bytes(), nil
	}

	switch exchange {
	case "P-224":
		curve := elliptic.P224()
		pub, priv, err := genKeyPair(curve)
		if err != nil {
			return nil, nil, err
		}

		done := func(theirs []byte) ([]byte, error) { return genSec(curve, theirs, priv) }

		return pub, done, nil

	case "P-256":
		curve := elliptic.P256()
		pub, priv, err := genKeyPair(curve)
		if err != nil {
			return nil, nil, err
		}

		done := func(theirs []byte) ([]byte, error) { return genSec(curve, theirs, priv) }

		return pub, done, nil

	case "P-384":
		curve := elliptic.P384()
		pub, priv, err := genKeyPair(curve)
		if err != nil {
			return nil, nil, err
		}

		done := func(theirs []byte) ([]byte, error) { return genSec(curve, theirs, priv) }

		return pub, done, nil

	case "P-521":
		curve := elliptic.P521()
		pub, priv, err := genKeyPair(curve)
		if err != nil {
			return nil, nil, err
		}

		done := func(theirs []byte) ([]byte, error) { return genSec(curve, theirs, priv) }

		return pub, done, nil

	}

	return nil, nil, errors.New("Something silly happened.")
}
