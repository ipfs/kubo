package pin

import (
	"context"
	"fmt"
	"os"
	"testing"

	dag "github.com/ipfs/go-ipfs/merkledag"
	mdtest "github.com/ipfs/go-ipfs/merkledag/test"

	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

func ignoreCids(_ *cid.Cid) {}

func TestSet(t *testing.T) {
	ds := mdtest.Mock()
	limit := 10000 // 10000 reproduces the pinloss issue fairly reliably

	if os.Getenv("STRESS_IT_OUT_YO") != "" {
		limit = 10000000
	}
	var inputs []*cid.Cid
	for i := 0; i < limit; i++ {
		c, err := ds.Add(dag.NodeWithData([]byte(fmt.Sprint(i))))
		if err != nil {
			t.Fatal(err)
		}

		inputs = append(inputs, c)
	}

	out, err := storeSet(context.Background(), ds, inputs, ignoreCids)
	if err != nil {
		t.Fatal(err)
	}

	// weird wrapper node because loadSet expects us to pass an
	// object pointing to multiple named sets
	setroot := &dag.ProtoNode{}
	err = setroot.AddNodeLinkClean("foo", out)
	if err != nil {
		t.Fatal(err)
	}

	outset, err := loadSet(context.Background(), ds, setroot, "foo", ignoreCids)
	if err != nil {
		t.Fatal(err)
	}

	if len(outset) != limit {
		t.Fatal("got wrong number", len(outset), limit)
	}

	seen := cid.NewSet()
	for _, c := range outset {
		seen.Add(c)
	}

	for _, c := range inputs {
		if !seen.Has(c) {
			t.Fatalf("expected to have %s, didnt find it")
		}
	}
}
