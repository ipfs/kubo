package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs/namesys"

	ipns "gx/ipfs/QmVHij7PuWUFeLcmRbD1ykDwB1WZMYP8yixo9bprUb3QHG/go-ipns"
	tu "gx/ipfs/QmXG74iiKQnDstVQq9fPFQEB6JTNSWBbAWE1qsq6L4E5sR/go-testutil"
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
