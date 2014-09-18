package daemon

import (
	"encoding/base64"
	"testing"

	config "github.com/jbenet/go-ipfs/config"
	core "github.com/jbenet/go-ipfs/core"
	ci "github.com/jbenet/go-ipfs/crypto"
	identify "github.com/jbenet/go-ipfs/identify"
)

func TestInitDaemonListener(t *testing.T) {

	priv, pub, err := ci.GenerateKeyPair(ci.RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	prbytes, err := priv.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	ident, _ := identify.IDFromPubKey(pub)
	privKey := base64.StdEncoding.EncodeToString(prbytes)
	pID := ident.Pretty()

	id := &config.Identity{
		PeerID:  pID,
		Address: "/ip4/127.0.0.1/tcp/8000",
		PrivKey: privKey,
	}

	nodeConfigs := []*config.Config{
		&config.Config{
			Identity: id,
			Datastore: config.Datastore{
				Type: "memory",
			},
		},

		&config.Config{
			Identity: id,
			Datastore: config.Datastore{
				Type: "leveldb",
				Path: ".testdb",
			},
		},
	}

	for _, c := range nodeConfigs {

		node, _ := core.NewIpfsNode(c, false)
		dl, initErr := NewDaemonListener(node, "localhost:1327")
		if initErr != nil {
			t.Fatal(initErr)
		}
		closeErr := dl.Close()
		if closeErr != nil {
			t.Fatal(closeErr)
		}

	}

}
