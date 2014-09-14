package crypto

import "testing"

func TestRsaKeys(t *testing.T) {
	sk, _, err := GenerateKeyPair(RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	testKeySignature(t, sk)
	testKeyEncoding(t, sk)
}

func testKeySignature(t *testing.T, sk PrivKey) {
	pk := sk.GetPublic()

	text := sk.GenSecret()
	sig, err := sk.Sign(text)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := pk.Verify(text, sig)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Fatal("Invalid signature.")
	}
}

func testKeyEncoding(t *testing.T, sk PrivKey) {
	skb, err := sk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	_, err = UnmarshalPrivateKey(skb)
	if err != nil {
		t.Fatal(err)
	}

	pk := sk.GetPublic()
	pkb, err := pk.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	_, err = UnmarshalPublicKey(pkb)
	if err != nil {
		t.Fatal(err)
	}
}
