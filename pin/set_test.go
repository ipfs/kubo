package pin

import (
	"context"
	"encoding/binary"
	"testing"

	bserv "gx/ipfs/QmQrLuAVriwcPQpqn15GU7gjZGKKa45Hmdj9JCG3Cc45CC/go-blockservice"
	dag "gx/ipfs/QmYxX4VfVcxmfsj8U6T5kVtFvHsSidy9tmPyPTW5fy7H3q/go-merkledag"

	offline "gx/ipfs/QmTMhpt5sH5yup2RU5qeFDMZcDf4RaHqba4ztEp7yrzese/go-ipfs-exchange-offline"
	ds "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore"
	dsq "gx/ipfs/QmVG5gxteQNEMhrS8prJSmU2C9rebtFuTd3SYZ5kE3YZ5k/go-datastore/query"
	cid "gx/ipfs/Qmdu2AYUV7yMoVBQPxXNfe7FJcdx16kYtsx6jAPKWQYF1y/go-cid"
	blockstore "gx/ipfs/QmeFZ47hGe5T8nSUjwd6zf6ikzFWYEzWsb1e4Q2r6n1w9z/go-ipfs-blockstore"
)

func ignoreCids(_ *cid.Cid) {}

func objCount(d ds.Datastore) int {
	q := dsq.Query{KeysOnly: true}
	res, err := d.Query(q)
	if err != nil {
		panic(err)
	}

	var count int
	for {
		_, ok := res.NextSync()
		if !ok {
			break
		}

		count++
	}
	return count
}

func TestSet(t *testing.T) {
	dst := ds.NewMapDatastore()
	bstore := blockstore.NewBlockstore(dst)
	ds := dag.NewDAGService(bserv.New(bstore, offline.Exchange(bstore)))

	// this value triggers the creation of a recursive shard.
	// If the recursive sharding is done improperly, this will result in
	// an infinite recursion and crash (OOM)
	limit := uint32((defaultFanout * maxItems) + 1)

	var inputs []*cid.Cid
	buf := make([]byte, 4)
	for i := uint32(0); i < limit; i++ {
		binary.BigEndian.PutUint32(buf, i)
		c := dag.NewRawNode(buf).Cid()
		inputs = append(inputs, c)
	}

	_, err := storeSet(context.Background(), ds, inputs[:len(inputs)-1], ignoreCids)
	if err != nil {
		t.Fatal(err)
	}

	objs1 := objCount(dst)

	out, err := storeSet(context.Background(), ds, inputs, ignoreCids)
	if err != nil {
		t.Fatal(err)
	}

	objs2 := objCount(dst)
	if objs2-objs1 > 2 {
		t.Fatal("set sharding does not appear to be deterministic")
	}

	// weird wrapper node because loadSet expects us to pass an
	// object pointing to multiple named sets
	setroot := &dag.ProtoNode{}
	err = setroot.AddNodeLink("foo", out)
	if err != nil {
		t.Fatal(err)
	}

	outset, err := loadSet(context.Background(), ds, setroot, "foo", ignoreCids)
	if err != nil {
		t.Fatal(err)
	}

	if uint32(len(outset)) != limit {
		t.Fatal("got wrong number", len(outset), limit)
	}

	seen := cid.NewSet()
	for _, c := range outset {
		seen.Add(c)
	}

	for _, c := range inputs {
		if !seen.Has(c) {
			t.Fatalf("expected to have '%s', didnt find it", c)
		}
	}
}
