package crypto

import (
	"errors"

	"crypto/rand"
	"crypto/rsa"

	"code.google.com/p/goprotobuf/proto"
)

var ErrBadKeyType = errors.New("invalid or unsupported key type")

const (
	RSA = iota
)

type PrivKey interface {
	// Cryptographically sign the given bytes
	Sign([]byte) ([]byte, error)

	// Decrypt a message encrypted with this keys public key
	Decrypt([]byte) ([]byte, error)

	// Return a public key paired with this private key
	GetPublic() PubKey

	// Generate a secret string of bytes
	GenSecret() []byte

	// Bytes returns a serialized, storeable representation of this key
	Bytes() ([]byte, error)
}

type PubKey interface {
	// Verify that 'sig' is the signed hash of 'data'
	Verify(data []byte, sig []byte) (bool, error)

	// Encrypt the given data with the public key
	Encrypt([]byte) ([]byte, error)

	// Bytes returns a serialized, storeable representation of this key
	Bytes() ([]byte, error)
}

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
