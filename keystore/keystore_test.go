package keystore

import (
	"os"
	"testing"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	u "github.com/ipfs/go-ipfs/util"
)

func TestKeyStorage(t *testing.T) {
	ksdir := os.TempDir()
	ks := NewFsKeystore(ksdir)

	k, _, err := ci.GenerateKeyPairWithReader(ci.RSA, 512, u.NewTimeSeededRand())
	if err != nil {
		t.Fatal(err)
	}

	err = ks.PutKey("testing", k)
	if err != nil {
		t.Fatal(err)
	}

	retk, err := ks.GetKey("testing")
	if err != nil {
		t.Fatal(err)
	}

	if !k.Equals(retk) {
		t.Fatal("keys were not equal!")
	}
}
