package record

import (
	"encoding/base64"
	"testing"

	key "github.com/ipfs/go-ipfs/blocks/key"
	ci "gx/ipfs/QmUWER4r4qMvaCnX5zREcfyiWN7cXN9g3a7fkRqNz8qWPP/go-libp2p-crypto"
)

var OffensiveKey = "CAASXjBcMA0GCSqGSIb3DQEBAQUAA0sAMEgCQQDjXAQQMal4SB2tSnX6NJIPmC69/BT8A8jc7/gDUZNkEhdhYHvc7k7S4vntV/c92nJGxNdop9fKJyevuNMuXhhHAgMBAAE="

func TestValidatePublicKey(t *testing.T) {
	pkb, err := base64.StdEncoding.DecodeString(OffensiveKey)
	if err != nil {
		t.Fatal(err)
	}

	pubk, err := ci.UnmarshalPublicKey(pkb)
	if err != nil {
		t.Fatal(err)
	}

	pkh, err := pubk.Hash()
	if err != nil {
		t.Fatal(err)
	}

	k := key.Key("/pk/" + string(pkh))

	err = ValidatePublicKeyRecord(k, pkb)
	if err != nil {
		t.Fatal(err)
	}
}
