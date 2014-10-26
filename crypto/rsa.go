package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	pb "github.com/jbenet/go-ipfs/crypto/internal/pb"
)

type RsaPrivateKey struct {
	k *rsa.PrivateKey
}

type RsaPublicKey struct {
	k *rsa.PublicKey
}

func (pk *RsaPublicKey) Verify(data, sig []byte) (bool, error) {
	hashed := sha256.Sum256(data)
	err := rsa.VerifyPKCS1v15(pk.k, crypto.SHA256, hashed[:], sig)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (pk *RsaPublicKey) Bytes() ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(pk.k)
	if err != nil {
		return nil, err
	}

	pbmes := new(pb.PublicKey)
	typ := pb.KeyType_RSA
	pbmes.Type = &typ
	pbmes.Data = b
	return proto.Marshal(pbmes)
}

func (pk *RsaPublicKey) Encrypt(b []byte) ([]byte, error) {
	return rsa.EncryptPKCS1v15(rand.Reader, pk.k, b)
}

// Equals checks whether this key is equal to another
func (pk *RsaPublicKey) Equals(k Key) bool {
	return KeyEqual(pk, k)
}

func (pk *RsaPublicKey) Hash() ([]byte, error) {
	return KeyHash(pk)
}

func (sk *RsaPrivateKey) GenSecret() []byte {
	buf := make([]byte, 16)
	rand.Read(buf)
	return buf
}

func (sk *RsaPrivateKey) Sign(message []byte) ([]byte, error) {
	hashed := sha256.Sum256(message)
	return rsa.SignPKCS1v15(rand.Reader, sk.k, crypto.SHA256, hashed[:])
}

func (sk *RsaPrivateKey) GetPublic() PubKey {
	return &RsaPublicKey{&sk.k.PublicKey}
}

func (sk *RsaPrivateKey) Unencrypt(b []byte) ([]byte, error) {
	return rsa.DecryptPKCS1v15(rand.Reader, sk.k, b)
}

func (sk *RsaPrivateKey) Bytes() ([]byte, error) {
	b := x509.MarshalPKCS1PrivateKey(sk.k)
	pbmes := new(pb.PrivateKey)
	typ := pb.KeyType_RSA
	pbmes.Type = &typ
	pbmes.Data = b
	return proto.Marshal(pbmes)
}

// Equals checks whether this key is equal to another
func (sk *RsaPrivateKey) Equals(k Key) bool {
	return KeyEqual(sk, k)
}

func (sk *RsaPrivateKey) Hash() ([]byte, error) {
	return KeyHash(sk)
}

func UnmarshalRsaPrivateKey(b []byte) (*RsaPrivateKey, error) {
	sk, err := x509.ParsePKCS1PrivateKey(b)
	if err != nil {
		return nil, err
	}
	return &RsaPrivateKey{sk}, nil
}

func UnmarshalRsaPublicKey(b []byte) (*RsaPublicKey, error) {
	pub, err := x509.ParsePKIXPublicKey(b)
	if err != nil {
		return nil, err
	}
	pk, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("Not actually an rsa public key.")
	}
	return &RsaPublicKey{pk}, nil
}
