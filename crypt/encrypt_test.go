package crypt

import (
	"bytes"
	"io/ioutil"
	"testing"

	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	u "github.com/ipfs/go-ipfs/util"
)

func TestEncryption(t *testing.T) {
	sk, pk, err := ci.GenerateKeyPairWithReader(ci.RSA, 512, u.NewTimeSeededRand())
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 1024*1024)
	u.NewTimeSeededRand().Read(data)
	r := bytes.NewReader(data)

	encr, err := EncryptStreamWithKey(r, pk)
	if err != nil {
		t.Fatal(err)
	}

	encbytes, err := ioutil.ReadAll(encr)
	if err != nil {
		t.Fatal(err)
	}

	todecrypt := bytes.NewReader(encbytes)

	decr, err := DecryptStreamWithKey(todecrypt, sk)
	if err != nil {
		t.Fatal(err)
	}

	plain, err := ioutil.ReadAll(decr)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, plain) {
		t.Fatal("output was not the same!")
	}
}
