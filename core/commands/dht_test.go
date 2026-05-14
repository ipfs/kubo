package commands

import (
	"testing"

	"github.com/ipfs/boxo/namesys"

	ipns "github.com/ipfs/boxo/ipns"
	"github.com/libp2p/go-libp2p/core/test"
)

func TestKeyTranslation(t *testing.T) {
	pid := test.RandPeerIDFatal(t)
	pkname := namesys.PkRoutingKey(pid)
	ipnsname := ipns.NameFromPeer(pid).RoutingKey()

	pkk, err := escapeDhtKey("/pk/" + pid.String())
	if err != nil {
		t.Fatal(err)
	}

	ipnsk, err := escapeDhtKey("/ipns/" + pid.String())
	if err != nil {
		t.Fatal(err)
	}

	if pkk != pkname {
		t.Fatal("keys didn't match!")
	}

	if ipnsk != string(ipnsname) {
		t.Fatal("keys didn't match!")
	}
}
