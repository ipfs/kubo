package peer

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	u "github.com/jbenet/go-ipfs/util"
)

func TestIDMatchesKey(t *testing.T) {
	p2, err := IDB58Decode("QmcNstKuwBBoVTpSCSDrwzjgrRcaYXK833Psuz2EMHwyQN")
	if err != nil {
		t.Fatal(err)
	}

	key := `CAASpgQwggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQC81JT2dK7zWjKWVAK4X
Csfuu2mjPssRopgF9zNOoP9fUPms1wtSwXKLMkXSSq8hBdk4LLem9X++DaDNu1uU/Focse/yeQ1XwJA
cwOyHtBfUrM2os4ViwXmpVcVfO4h6ZcqDebxfb30aXpjvk0ujbtC//zpHp4pRfClq0Q/f/pQaKJbrR6
3jodY8g77uLP5LOUmV/e2L0KuF/mluzNMAZU5dtBMwUBTBWEDTvRyU4S9BrgJimaIX7Szrr7SpA/Zcv
HRYclIramjhT1g9lcJkgS/MHgfm961AURprA4VvhG/QuBhXTFH187Pn2Ru8q7zANtmbdlsfgu2zpjfn
2B4aBBtJcGlKfiVcIiLR4ZoZZO+YadNbZPgA8MMJue8pA+KgaMSkz06pM3PB8d29RdkvsU9Mb9gbjlc
OeMwlJ9+XhKqFq4q7NRA9syH8ehZLAdPXZHHoqhvCoFgWUNoWeofcw6Rgq4S2T2xdwyj1wDAlSOpFFZ
yl05aJBxK8Qc2u6DXDdR3kLBpgMNYFwDosmY4imLNVUZCG2qW9X3PjyWvwCq3EXihY+64em/FIOjfPU
IRby5H1QoB7/HmsfQH5ctpxa+8xRxiVQJc90J8YT6xjWPSiTPHA6Dv00+e8aq3gDoPDqqTiv5CixP+r
7oKHR/QOHGiq2wlW7tk19gUuAD9KQDBxwIDAQAB`
	key = strings.Replace(key, "\n", "", -1)

	keyb, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		t.Fatal(err)
	}

	// cast is using multihash Cast
	hsh := u.Hash(keyb)
	if !bytes.Equal(hsh, []byte(p2)) {
		t.Error("peerID and key should match")
	}
}
