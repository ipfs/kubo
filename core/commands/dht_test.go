package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs/namesys"

	ipns "gx/ipfs/QmQZJ3WAhrhGXrw7DWNJpAvwoEg2JCi8aR2DjYMZHaX1h4/go-ipns"
	tu "gx/ipfs/QmbtxgRVMQS2fZpuwowZRQUg3QNri9yuHSFGfXUrqoSABU/go-testutil"
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
