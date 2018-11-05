package pin

import (
	"context"
	"encoding/binary"
	"testing"

	bserv "gx/ipfs/QmVPeMNK9DfGLXDZzs2W4RoFWC9Zq1EnLGmLXtYtWrNdcW/go-blockservice"
	dag "gx/ipfs/QmaDBne4KeY3UepeqSVKYpSmQGa3q9zP6x3LfVF2UjF3Hc/go-merkledag"

	offline "gx/ipfs/QmPpnbwgAuvhUkA9jGooR88ZwZtTUHXXvoQNKdjZC6nYku/go-ipfs-exchange-offline"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	blockstore "gx/ipfs/QmSNLNnL3kq3A1NGdQA9AtgxM9CWKiiSEup3W435jCkRQS/go-ipfs-blockstore"
	ds "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore"
	dsq "gx/ipfs/QmaRb5yNXKonhbkpNxNawoydk4N6es6b4fPj19sjEKsh5D/go-datastore/query"
)

func ignoreCids(_ cid.Cid) {}

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

	var inputs []cid.Cid
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
