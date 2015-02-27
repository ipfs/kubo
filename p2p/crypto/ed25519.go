package crypto

import (
	"errors"
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/agl/ed25519"
	"io"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	pb "github.com/jbenet/go-ipfs/p2p/crypto/internal/pb"
)

type Ed25519PrivateKey struct {
	sk *[ed25519.PrivateKeySize]byte
	pk *[ed25519.PublicKeySize]byte
}

type Ed25519PublicKey struct {
	k *[ed25519.PublicKeySize]byte
}

func generateEd25519KeyPair(src io.Reader) (PrivKey, PubKey, error) {
	pk, sk, err := ed25519.GenerateKey(src)
	if err != nil {
		return nil, nil, err
	}
	return &Ed25519PrivateKey{sk, pk}, &Ed25519PublicKey{pk}, nil
}

func (pk *Ed25519PublicKey) Verify(data, sig []byte) (bool, error) {
	if len(sig) != 64 {
		return false, errors.New("Signature must be 64 bytes long")
	}
	var sigarray [64]byte
	copy(sigarray[:], sig)
	return ed25519.Verify(pk.k, data, &sigarray), nil
}

func (pk *Ed25519PublicKey) Bytes() ([]byte, error) {
	pbmes := new(pb.PublicKey)
	typ := pb.KeyType_Ed25519
	pbmes.Type = &typ
	pbmes.Data = pk.k[:]
	return proto.Marshal(pbmes)
}

// Equals checks whether this key is equal to another
func (pk *Ed25519PublicKey) Equals(k Key) bool {
	return KeyEqual(pk, k)
}

func (pk *Ed25519PublicKey) Hash() ([]byte, error) {
	return KeyHash(pk)
}

func (sk *Ed25519PrivateKey) Sign(message []byte) ([]byte, error) {
	return ed25519.Sign(sk.sk, message)[:], nil
}

func (sk *Ed25519PrivateKey) GetPublic() PubKey {
	return &Ed25519PublicKey{sk.pk}
}

func (sk *Ed25519PrivateKey) Bytes() ([]byte, error) {
	pbmes := new(pb.PrivateKey)
	typ := pb.KeyType_Ed25519
	pbmes.Type = &typ
	pbmes.Data = append(sk.sk[:], sk.pk[:]...)
	return proto.Marshal(pbmes)
}

// Equals checks whether this key is equal to another
func (sk *Ed25519PrivateKey) Equals(k Key) bool {
	return KeyEqual(sk, k)
}

func (sk *Ed25519PrivateKey) Hash() ([]byte, error) {
	return KeyHash(sk)
}

func UnmarshalEd25519PublicKey(b []byte) (*Ed25519PublicKey, error) {
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("Public key must be %d bytes long", ed25519.PublicKeySize)
	}
	var pk Ed25519PublicKey
	pk.k = new([ed25519.PublicKeySize]byte)
	copy(pk.k[:], b)
	return &pk, nil
}

func UnmarshalEd25519PrivateKey(b []byte) (*Ed25519PrivateKey, error) {
	if len(b) != ed25519.PublicKeySize+ed25519.PrivateKeySize {
		return nil, fmt.Errorf("Private key must be %d bytes long", ed25519.PublicKeySize+ed25519.PrivateKeySize)
	}
	var sk Ed25519PrivateKey
	sk.sk = new([ed25519.PrivateKeySize]byte)
	sk.pk = new([ed25519.PublicKeySize]byte)
	copy(sk.sk[:], b[:ed25519.PrivateKeySize])
	copy(sk.pk[:], b[ed25519.PrivateKeySize:])
	return &sk, nil
}
