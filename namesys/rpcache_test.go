package namesys

import (
	"testing"

	path "github.com/ipfs/go-ipfs/path"

	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	dssync "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore/sync"
	testutil "gx/ipfs/QmVvkK7s5imCiq3JVbL3pGfnhcCnf3LrFJPF4GE2sAoGZf/go-testutil"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

func TestRPCachePutGet(t *testing.T) {
	dstore := dssync.MutexWrap(ds.NewMapDatastore())
	p := path.FromString("/ipfs/QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN")

	_, pubk, err := testutil.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	pid, err := peer.IDFromPublicKey(pubk)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = RepubCacheGet(dstore, pid)
	if err != ds.ErrNotFound {
		t.Fatalf("Expected ds.ErrNotFound but got %v", err)
	}

	seqnum := uint64(5)
	err = RepubCachePut(dstore, pid, seqnum, p)
	if err != nil {
		t.Fatal(err)
	}

	gseqnum, gp, err := RepubCacheGet(dstore, pid)
	if err != nil {
		t.Fatal(err)
	}

	if seqnum != gseqnum {
		t.Fatalf("Sequence number mismatch: %d != %d", seqnum, gseqnum)
	}
	if p != gp {
		t.Fatalf("Path mismatch: '%s' != '%s'", p, gp)
	}
}
