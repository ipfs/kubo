package namesys

import (
	"testing"
	"time"

	path "github.com/ipfs/go-ipfs/path"
	u "gx/ipfs/QmPsAfmDBnZN3kZGSuNwvCNDZiHneERSKmRcFyG3UkvcT3/go-ipfs-util"
	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"
	ci "gx/ipfs/QmaPbCnUMBohSGo3KnxEa2bHqyJVVeEEcwtqJAYxerieBo/go-libp2p-crypto"
	record "gx/ipfs/QmbsY8Pr6s3uZsKg7rzBtGDKeCtdoAhNaMTCXBUbvb1eCV/go-libp2p-record"
)

func TestValidation(t *testing.T) {
	// Create a record validator
	validator := make(record.Validator)
	validator["ipns"] = &record.ValidChecker{ValidateIpnsRecord, true}

	// Generate a key for signing the records
	r := u.NewSeededRand(15) // generate deterministic keypair
	priv, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, r)
	if err != nil {
		t.Fatal(err)
	}

	// Create entry with expiry in one hour
	ts := time.Now()
	entry, err := CreateRoutingEntryData(priv, path.Path("foo"), 1, ts.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	// Get IPNS record path
	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pubkh := u.Hash(pubkb).B58String()
	ipnsPath := "/ipns/" + pubkh

	val, err := proto.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}

	// Create the record
	r1, err := record.MakePutRecord(priv, ipnsPath, val, true)

	// Validate the record
	err = validator.VerifyRecord(r1)
	if err != nil {
		t.Fatal(err)
	}

	// Create IPNS record path with a different key
	_, pubk2, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, r)
	if err != nil {
		t.Fatal(err)
	}
	pubkb2, err := pubk2.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pubkh2 := u.Hash(pubkb2).B58String()
	ipnsWrongPath := "/ipns/" + pubkh2

	r2, err := record.MakePutRecord(priv, ipnsWrongPath, val, true)

	// Record should fail validation because path doesn't match author
	err = validator.VerifyRecord(r2)
	if err != ErrInvalidAuthor {
		t.Fatal("ValidateIpnsRecord should have returned ErrInvalidAuthor")
	}

	// Create expired entry
	expired, err := CreateRoutingEntryData(priv, path.Path("foo"), 1, ts.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	valExp, err := proto.Marshal(expired)
	if err != nil {
		t.Fatal(err)
	}

	// Create record with the expired entry
	r3, err := record.MakePutRecord(priv, ipnsPath, valExp, true)

	// Record should fail validation because entry is expired
	err = validator.VerifyRecord(r3)
	if err != ErrExpiredRecord {
		t.Fatal("ValidateIpnsRecord should have returned ErrExpiredRecord")
	}
}
