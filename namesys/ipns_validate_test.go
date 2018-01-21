package namesys

import (
	"io"
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
	priv, ipnsPath := genKeys(t, r)

	// Create entry with expiry in one hour
	ts := time.Now()
	entry, err := CreateRoutingEntryData(priv, path.Path("foo"), 1, ts.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	val, err := proto.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}

	// Create the record
	rec, err := record.MakePutRecord(priv, ipnsPath, val, true)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the record
	err = validator.VerifyRecord(rec)
	if err != nil {
		t.Fatal(err)
	}


	// Create IPNS record path with a different key
	_, ipnsWrongAuthor := genKeys(t, r)
	wrongAuthorRec, err := record.MakePutRecord(priv, ipnsWrongAuthor, val, true)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because path doesn't match author
	err = validator.VerifyRecord(wrongAuthorRec)
	if err != ErrInvalidAuthor {
		t.Fatal("ValidateIpnsRecord should have returned ErrInvalidAuthor")
	}


	// Create IPNS record path with extra path components after author
	extraPath := ipnsPath + "/some/path"
	extraPathRec, err := record.MakePutRecord(priv, extraPath, val, true)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because path has extra components after author
	err = validator.VerifyRecord(extraPathRec)
	if err != ErrInvalidAuthor {
		t.Fatal("ValidateIpnsRecord should have returned ErrInvalidAuthor")
	}


	// Create unsigned IPNS record
	unsignedRec, err := record.MakePutRecord(priv, ipnsPath, val, false)
	if err != nil {
		t.Fatal(err)
	}

	// Record should fail validation because IPNS records require signature
	err = validator.VerifyRecord(unsignedRec)
	if err != ErrInvalidAuthor {
		t.Fatal("ValidateIpnsRecord should have returned ErrInvalidAuthor")
	}


	// Create unsigned IPNS record with no author
	unsignedRecNoAuthor, err := record.MakePutRecord(priv, ipnsPath, val, false)
	if err != nil {
		t.Fatal(err)
	}
	noAuth := ""
	unsignedRecNoAuthor.Author = &noAuth

	// Record should fail validation because IPNS records require author
	err = validator.VerifyRecord(unsignedRecNoAuthor)
	if err != ErrInvalidAuthor {
		t.Fatal("ValidateIpnsRecord should have returned ErrInvalidAuthor")
	}


	// Create expired entry
	expiredEntry, err := CreateRoutingEntryData(priv, path.Path("foo"), 1, ts.Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	valExp, err := proto.Marshal(expiredEntry)
	if err != nil {
		t.Fatal(err)
	}

	// Create record with the expired entry
	expiredRec, err := record.MakePutRecord(priv, ipnsPath, valExp, true)

	// Record should fail validation because entry is expired
	err = validator.VerifyRecord(expiredRec)
	if err != ErrExpiredRecord {
		t.Fatal("ValidateIpnsRecord should have returned ErrExpiredRecord")
	}
}

func genKeys(t *testing.T, r io.Reader) (ci.PrivKey, string) {
	priv, pubk, err := ci.GenerateKeyPairWithReader(ci.RSA, 1024, r)
	if err != nil {
		t.Fatal(err)
	}
	pubkb, err := pubk.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	p := "/ipns/" + u.Hash(pubkb).B58String()
	return priv, p
}
