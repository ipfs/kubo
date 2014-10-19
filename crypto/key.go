package crypto

import (
	"bytes"
	"errors"
	"fmt"

	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("crypto")

var ErrBadKeyType = errors.New("invalid or unsupported key type")

const (
	RSA = iota
)

type Key interface {
	// Bytes returns a serialized, storeable representation of this key
	Bytes() ([]byte, error)

	// Hash returns the hash of this key
	Hash() ([]byte, error)

	// Equals checks whether two PubKeys are the same
	Equals(Key) bool
}

type PrivKey interface {
	Key

	// Cryptographically sign the given bytes
	Sign([]byte) ([]byte, error)

	// Return a public key paired with this private key
	GetPublic() PubKey

	// Generate a secret string of bytes
	GenSecret() []byte
}

type PubKey interface {
	Key

	// Verify that 'sig' is the signed hash of 'data'
	Verify(data []byte, sig []byte) (bool, error)
}

// Given a public key, generates the shared key.
type GenSharedKey func([]byte) ([]byte, error)

func GenerateKeyPair(typ, bits int) (PrivKey, PubKey, error) {
	switch typ {
	case RSA:
		priv, err := rsa.GenerateKey(rand.Reader, bits)
		if err != nil {
			return nil, nil, err
		}
		pk := &priv.PublicKey
		return &RsaPrivateKey{priv}, &RsaPublicKey{pk}, nil
	default:
		return nil, nil, ErrBadKeyType
	}
}

// Generates an ephemeral public key and returns a function that will compute
// the shared secret key.  Used in the identify module.
//
// Focuses only on ECDH now, but can be made more general in the future.
func GenerateEKeyPair(curveName string) ([]byte, GenSharedKey, error) {
	var curve elliptic.Curve

	switch curveName {
	case "P-224":
		curve = elliptic.P224()
	case "P-256":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	}

	priv, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	pubKey := elliptic.Marshal(curve, x, y)
	// log.Debug("GenerateEKeyPair %d", len(pubKey))

	done := func(theirPub []byte) ([]byte, error) {
		// Verify and unpack node's public key.
		x, y := elliptic.Unmarshal(curve, theirPub)
		if x == nil {
			return nil, fmt.Errorf("Malformed public key: %d %v", len(theirPub), theirPub)
		}

		if !curve.IsOnCurve(x, y) {
			return nil, errors.New("Invalid public key.")
		}

		// Generate shared secret.
		secret, _ := curve.ScalarMult(x, y, priv)

		return secret.Bytes(), nil
	}

	return pubKey, done, nil
}

// Generates a set of keys for each party by stretching the shared key.
// (myIV, theirIV, myCipherKey, theirCipherKey, myMACKey, theirMACKey)
func KeyStretcher(cmp int, cipherType string, hashType string, secret []byte) ([]byte, []byte, []byte, []byte, []byte, []byte) {
	var cipherKeySize int
	switch cipherType {
	case "AES-128":
		cipherKeySize = 16
	case "AES-256":
		cipherKeySize = 32
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

func UnmarshalPublicKey(data []byte) (PubKey, error) {
	pmes := new(PBPublicKey)
	err := proto.Unmarshal(data, pmes)
	if err != nil {
		return nil, err
	}

	switch pmes.GetType() {
	case KeyType_RSA:
		return UnmarshalRsaPublicKey(pmes.GetData())
	default:
		return nil, ErrBadKeyType
	}
}

func UnmarshalPrivateKey(data []byte) (PrivKey, error) {
	pmes := new(PBPrivateKey)
	err := proto.Unmarshal(data, pmes)
	if err != nil {
		return nil, err
	}

	switch pmes.GetType() {
	case KeyType_RSA:
		return UnmarshalRsaPrivateKey(pmes.GetData())
	default:
		return nil, ErrBadKeyType
	}
}

// KeyEqual checks whether two
func KeyEqual(k1, k2 Key) bool {
	if k1 == k2 {
		return true
	}

	b1, err1 := k1.Bytes()
	b2, err2 := k2.Bytes()
	return bytes.Equal(b1, b2) && err1 == err2
}

// KeyHash hashes a key.
func KeyHash(k Key) ([]byte, error) {
	kb, err := k.Bytes()
	if err != nil {
		return nil, err
	}
	return u.Hash(kb), nil
}
