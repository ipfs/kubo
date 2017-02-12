package commands

import (
	"testing"

	"github.com/ipfs/go-ipfs/namesys"
	tu "github.com/ipfs/go-ipfs/thirdparty/testutil"
	routing "gx/ipfs/QmbkGVaN9W6RYJK4Ws5FvMKXKDqdRQ5snhtaa92qP6L8eU/go-libp2p-routing"
)

func TestKeyTranslation(t *testing.T) {
	pid := tu.RandPeerIDFatal(t)
	a := routing.KeyForPublicKey(pid)
	b := namesys.IpnsKeyForID(pid)

	pkk, err := escapeDhtKey("/pk/" + pid.Pretty())
	if err != nil {
		t.Fatal(err)
	}

	ipnsk, err := escapeDhtKey("/ipns/" + pid.Pretty())
	if err != nil {
		t.Fatal(err)
	}

	if pkk != a {
		t.Fatal("keys didnt match!")
	}

	if ipnsk != b {
		t.Fatal("keys didnt match!")
	}
}
