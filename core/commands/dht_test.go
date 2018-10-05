package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs/namesys"

	tu "gx/ipfs/QmNfQbgBfARAtrYsBguChX6VJ5nbjeoYy1KdC36aaYWqG8/go-testutil"
	ipns "gx/ipfs/QmWhm9qS3NZdvTgAsB1cX4q9UbNu8yFybCcAMchCN88w7o/go-ipns"
)

func TestKeyTranslation(t *testing.T) {
	pid := tu.RandPeerIDFatal(t)
	pkname := namesys.PkKeyForID(pid)
	ipnsname := ipns.RecordKey(pid)

	pkk, err := escapeDhtKey("/pk/" + pid.Pretty())
	if err != nil {
		t.Fatal(err)
	}

	ipnsk, err := escapeDhtKey("/ipns/" + pid.Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if pkk != pkname {
		t.Fatal("keys didnt match!")
	}

	if ipnsk != ipnsname {
		t.Fatal("keys didnt match!")
	}
}
