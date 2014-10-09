package crypto

import "testing"

func TestRsaKeys(t *testing.T) {
	sk, pk, err := GenerateKeyPair(RSA, 512)
	if err != nil {
		t.Fatal(err)
	}
	testKeySignature(t, sk)
	testKeyEncoding(t, sk)
	testKeyEquals(t, sk)
	testKeyEquals(t, pk)
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

func testKeyEquals(t *testing.T, k Key) {
	kb, err := k.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	if !KeyEqual(k, k) {
		t.Fatal("Key not equal to itself.")
	}

	if !KeyEqual(k, testkey(kb)) {
		t.Fatal("Key not equal to key with same bytes.")
	}

	sk, pk, err := GenerateKeyPair(RSA, 512)
	if err != nil {
		t.Fatal(err)
	}

	if KeyEqual(k, sk) {
		t.Fatal("Keys should not equal.")
	}

	if KeyEqual(k, pk) {
		t.Fatal("Keys should not equal.")
	}
}

type testkey []byte

func (pk testkey) Bytes() ([]byte, error) {
	return pk, nil
}

func (pk testkey) Equals(k Key) bool {
	return KeyEqual(pk, k)
}

func (pk testkey) Hash() ([]byte, error) {
	return KeyHash(pk)
}
