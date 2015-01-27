package testutil

import (
	"bytes"
	"io"
	"testing"

	eventlog "github.com/jbenet/go-ipfs/thirdparty/eventlog"
	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"

	ic "github.com/jbenet/go-ipfs/p2p/crypto"
	peer "github.com/jbenet/go-ipfs/p2p/peer"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

var log = eventlog.Logger("boguskey")

// TestBogusPrivateKey is a key used for testing (to avoid expensive keygen)
type TestBogusPrivateKey []byte

// TestBogusPublicKey is a key used for testing (to avoid expensive keygen)
type TestBogusPublicKey []byte

func (pk TestBogusPublicKey) Verify(data, sig []byte) (bool, error) {
	log.Criticalf("TestBogusPublicKey.Verify -- this better be a test!")
	return bytes.Equal(data, reverse(sig)), nil
}

func (pk TestBogusPublicKey) Bytes() ([]byte, error) {
	return []byte(pk), nil
}

func (pk TestBogusPublicKey) Encrypt(b []byte) ([]byte, error) {
	log.Criticalf("TestBogusPublicKey.Encrypt -- this better be a test!")
	return reverse(b), nil
}

// Equals checks whether this key is equal to another
func (pk TestBogusPublicKey) Equals(k ic.Key) bool {
	return ic.KeyEqual(pk, k)
}

func (pk TestBogusPublicKey) Hash() ([]byte, error) {
	return ic.KeyHash(pk)
}

func (sk TestBogusPrivateKey) GenSecret() ([]byte, error) {
	return []byte(sk), nil
}

func (sk TestBogusPrivateKey) Sign(message []byte) ([]byte, error) {
	log.Criticalf("TestBogusPrivateKey.Sign -- this better be a test!")
	return reverse(message), nil
}

func (sk TestBogusPrivateKey) GetPublic() ic.PubKey {
	return TestBogusPublicKey(sk)
}

func (sk TestBogusPrivateKey) Decrypt(b []byte) ([]byte, error) {
	log.Criticalf("TestBogusPrivateKey.Decrypt -- this better be a test!")
	return reverse(b), nil
}

func (sk TestBogusPrivateKey) Bytes() ([]byte, error) {
	return []byte(sk), nil
}

// Equals checks whether this key is equal to another
func (sk TestBogusPrivateKey) Equals(k ic.Key) bool {
	return ic.KeyEqual(sk, k)
}

func (sk TestBogusPrivateKey) Hash() ([]byte, error) {
	return ic.KeyHash(sk)
}

func RandTestBogusPrivateKey() (TestBogusPrivateKey, error) {
	r := u.NewTimeSeededRand()
	k := make([]byte, 5)
	if _, err := io.ReadFull(r, k); err != nil {
		return nil, err
	}
	return TestBogusPrivateKey(k), nil
}

func RandTestBogusPublicKey() (TestBogusPublicKey, error) {
	k, err := RandTestBogusPrivateKey()
	return TestBogusPublicKey(k), err
}

func RandTestBogusPrivateKeyOrFatal(t *testing.T) TestBogusPrivateKey {
	k, err := RandTestBogusPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func RandTestBogusPublicKeyOrFatal(t *testing.T) TestBogusPublicKey {
	k, err := RandTestBogusPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func RandTestBogusIdentity() (testutil.Identity, error) {
	k, err := RandTestBogusPrivateKey()
	if err != nil {
		return nil, err
	}

	id, err := peer.IDFromPrivateKey(k)
	if err != nil {
		return nil, err
	}

	return &identity{
		k:  k,
		id: id,
		a:  testutil.RandLocalTCPAddress(),
	}, nil
}

func RandTestBogusIdentityOrFatal(t *testing.T) testutil.Identity {
	k, err := RandTestBogusIdentity()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// identity is a temporary shim to delay binding of PeerNetParams.
type identity struct {
	k  TestBogusPrivateKey
	id peer.ID
	a  ma.Multiaddr
}

func (p *identity) ID() peer.ID {
	return p.id
}

func (p *identity) Address() ma.Multiaddr {
	return p.a
}

func (p *identity) PrivateKey() ic.PrivKey {
	return p.k
}

func (p *identity) PublicKey() ic.PubKey {
	return p.k.GetPublic()
}

func reverse(a []byte) []byte {
	b := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		b[i] = a[len(a)-1-i]
	}
	return b
}
