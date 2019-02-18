package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs/namesys"

	ipns "gx/ipfs/QmdboayjE53q27kq6zGk5vkx4u7LDGdcbVoi5NXMCtfiKS/go-ipns"
	tu "gx/ipfs/QmeFVdhzY13YZPWxCiQvmLercrumFRoQZFQEYw2BtzyiQc/go-testutil"
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
