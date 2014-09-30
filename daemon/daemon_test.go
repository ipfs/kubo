package daemon

import (
	"encoding/base64"
	"os"
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	spipe "github.com/jbenet/go-ipfs/crypto/spipe"
)

func TestInitializeDaemonListener(t *testing.T) {

	priv, pub, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	prbytes, err := priv.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	ident, _ := spipe.IDFromPubKey(pub)
	privKey := base64.StdEncoding.EncodeToString(prbytes)
	pID := ident.Pretty()

	id := config.Identity{
		PeerID:  pID,
		PrivKey: privKey,
	}

	nodeConfigs := []*config.Config{
		&config.Config{
			Identity: id,
			Datastore: config.Datastore{
				Type: "memory",
			},
			Addresses: config.Addresses{
				Swarm: "/ip4/0.0.0.0/tcp/4001",
				API:   "/ip4/127.0.0.1/tcp/8000",
			},
		},

		&config.Config{
			Identity: id,
			Datastore: config.Datastore{
				Type: "leveldb",
				Path: ".test/datastore",
			},
			Addresses: config.Addresses{
				Swarm: "/ip4/0.0.0.0/tcp/4001",
				API:   "/ip4/127.0.0.1/tcp/8000",
			},
		},
	}

	var tempConfigDir = ".test"
	err = os.MkdirAll(tempConfigDir, os.ModeDir|0777)
	if err != nil {
		t.Fatalf("error making temp config dir: %v", err)
	}

	for _, c := range nodeConfigs {

		node, _ := core.NewIpfsNode(c, false)
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/1327")
		if err != nil {
			t.Fatal(err)
		}

		dl, initErr := NewDaemonListener(node, addr, tempConfigDir)
		if initErr != nil {
			t.Fatal(initErr)
		}

		closeErr := dl.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}

	}

}
