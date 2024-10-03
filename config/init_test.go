package config

import (
	"bytes"
	"testing"

	"github.com/ipfs/kubo/core/coreiface/options"
	crypto_pb "github.com/libp2p/go-libp2p/core/crypto/pb"
)

func TestCreateIdentity(t *testing.T) {
	writer := bytes.NewBuffer(nil)
	id, err := CreateIdentity(writer, []options.KeyGenerateOption{options.Key.Type(options.Ed25519Key)})
	if err != nil {
		t.Fatal(err)
	}
	pk, err := id.DecodePrivateKey("")
	if err != nil {
		t.Fatal(err)
	}
	if pk.Type() != crypto_pb.KeyType_Ed25519 {
		t.Fatal("unexpected type:", pk.Type())
	}

	id, err = CreateIdentity(writer, []options.KeyGenerateOption{options.Key.Type(options.RSAKey)})
	if err != nil {
		t.Fatal(err)
	}
	pk, err = id.DecodePrivateKey("")
	if err != nil {
		t.Fatal(err)
	}
	if pk.Type() != crypto_pb.KeyType_RSA {
		t.Fatal("unexpected type:", pk.Type())
	}
}

func TestCreateIdentityOptions(t *testing.T) {
	var w bytes.Buffer

	// ed25519 keys with bit size must fail.
	_, err := CreateIdentity(&w, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
		options.Key.Size(2048),
	})
	if err == nil {
		t.Errorf("ed25519 keys cannot have a custom bit size")
	}
}
