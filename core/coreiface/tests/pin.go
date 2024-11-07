package tests

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
	iface "github.com/ipfs/kubo/core/coreiface"
	opt "github.com/ipfs/kubo/core/coreiface/options"
)

func (tp *TestSuite) TestPin(t *testing.T) {
	tp.hasApi(t, func(api iface.CoreAPI) error {
		if api.Pin() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestPinAdd", tp.TestPinAdd)
	t.Run("TestPinSimple", tp.TestPinSimple)
	t.Run("TestPinRecursive", tp.TestPinRecursive)
	t.Run("TestPinLsIndirect", tp.TestPinLsIndirect)
	t.Run("TestPinLsPrecedence", tp.TestPinLsPrecedence)
	t.Run("TestPinIsPinned", tp.TestPinIsPinned)
}

func (tp *TestSuite) TestPinAdd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
}

func (tp *TestSuite) TestPinSimple(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	list, err := accPins(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().RootCid().String() != p.RootCid().String() {
		t.Error("paths don't match")
	}

	if list[0].Type() != "recursive" {
		t.Error("unexpected pin type")
	}

	assertIsPinned(t, ctx, api, p, "recursive")

	err = api.Pin().Rm(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	list, err = accPins(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 0 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}
}

func (tp *TestSuite) TestPinRecursive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	p0, err := api.Unixfs().Add(ctx, strFile("foo")())
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Unixfs().Add(ctx, strFile("bar")())
	if err != nil {
		t.Fatal(err)
	}

	nd2, err := ipldcbor.FromJSON(strings.NewReader(`{"lnk": {"/": "`+p0.RootCid().String()+`"}}`), math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	nd3, err := ipldcbor.FromJSON(strings.NewReader(`{"lnk": {"/": "`+p1.RootCid().String()+`"}}`), math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := api.Dag().AddMany(ctx, []ipld.Node{nd2, nd3}); err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(nd2.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(nd3.Cid()), opt.Pin.Recursive(false))
	if err != nil {
		t.Fatal(err)
	}

	list, err := accPins(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 3 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	list, err = accPins(ctx, api, opt.Pin.Ls.Direct())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != path.FromCid(nd3.Cid()).String() {
		t.Errorf("unexpected path, %s != %s", list[0].Path().String(), path.FromCid(nd3.Cid()).String())
	}

	list, err = accPins(ctx, api, opt.Pin.Ls.Recursive())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().String() != path.FromCid(nd2.Cid()).String() {
		t.Errorf("unexpected path, %s != %s", list[0].Path().String(), path.FromCid(nd2.Cid()).String())
	}

	list, err = accPins(ctx, api, opt.Pin.Ls.Indirect())
	if err != nil {
		t.Fatal(err)
	}

	if len(list) != 1 {
		t.Errorf("unexpected pin list len: %d", len(list))
	}

	if list[0].Path().RootCid().String() != p0.RootCid().String() {
		t.Errorf("unexpected path, %s != %s", list[0].Path().RootCid().String(), p0.RootCid().String())
	}

	res, err := api.Pin().Verify(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for r := range res {
		if err := r.Err(); err != nil {
			t.Error(err)
		}
		if !r.Ok() {
			t.Error("expected pin to be ok")
		}
		n++
	}

	if n != 1 {
		t.Errorf("unexpected verify result count: %d", n)
	}

	// TODO: figure out a way to test verify without touching IpfsNode
	/*
		err = api.Block().Rm(ctx, p0, opt.Block.Force(true))
		if err != nil {
			t.Fatal(err)
		}

		res, err = api.Pin().Verify(ctx)
		if err != nil {
			t.Fatal(err)
		}
		n = 0
		for r := range res {
			if r.Ok() {
				t.Error("expected pin to not be ok")
			}

			if len(r.BadNodes()) != 1 {
				t.Fatalf("unexpected badNodes len")
			}

			if r.BadNodes()[0].Path().Cid().String() != p0.Cid().String() {
				t.Error("unexpected badNode path")
			}

			if r.BadNodes()[0].Err().Error() != "merkledag: not found" {
				t.Errorf("unexpected badNode error: %s", r.BadNodes()[0].Err().Error())
			}
			n++
		}

		if n != 1 {
			t.Errorf("unexpected verify result count: %d", n)
		}
	*/
}

// TestPinLsIndirect verifies that indirect nodes are listed by pin ls even if a parent node is directly pinned
func (tp *TestSuite) TestPinLsIndirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	leaf, parent, grandparent := getThreeChainedNodes(t, ctx, api, "foo")

	err = api.Pin().Add(ctx, path.FromCid(grandparent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(parent.Cid()), opt.Pin.Recursive(false))
	if err != nil {
		t.Fatal(err)
	}

	assertPinTypes(t, ctx, api, []cidContainer{grandparent}, []cidContainer{parent}, []cidContainer{leaf})
}

// TestPinLsPrecedence verifies the precedence of pins (recursive > direct > indirect)
func (tp *TestSuite) TestPinLsPrecedence(t *testing.T) {
	// Testing precedence of recursive, direct and indirect pins
	// Results should be recursive > indirect, direct > indirect, and recursive > direct

	t.Run("TestPinLsPredenceRecursiveIndirect", tp.TestPinLsPredenceRecursiveIndirect)
	t.Run("TestPinLsPrecedenceDirectIndirect", tp.TestPinLsPrecedenceDirectIndirect)
	t.Run("TestPinLsPrecedenceRecursiveDirect", tp.TestPinLsPrecedenceRecursiveDirect)
}

func (tp *TestSuite) TestPinLsPredenceRecursiveIndirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test recursive > indirect
	leaf, parent, grandparent := getThreeChainedNodes(t, ctx, api, "recursive > indirect")

	err = api.Pin().Add(ctx, path.FromCid(grandparent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(parent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	assertPinTypes(t, ctx, api, []cidContainer{grandparent, parent}, []cidContainer{}, []cidContainer{leaf})
}

func (tp *TestSuite) TestPinLsPrecedenceDirectIndirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test direct > indirect
	leaf, parent, grandparent := getThreeChainedNodes(t, ctx, api, "direct > indirect")

	err = api.Pin().Add(ctx, path.FromCid(grandparent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(parent.Cid()), opt.Pin.Recursive(false))
	if err != nil {
		t.Fatal(err)
	}

	assertPinTypes(t, ctx, api, []cidContainer{grandparent}, []cidContainer{parent}, []cidContainer{leaf})
}

func (tp *TestSuite) TestPinLsPrecedenceRecursiveDirect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test recursive > direct
	leaf, parent, grandparent := getThreeChainedNodes(t, ctx, api, "recursive + direct = error")

	err = api.Pin().Add(ctx, path.FromCid(parent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(parent.Cid()), opt.Pin.Recursive(false))
	if err == nil {
		t.Fatal("expected error directly pinning a recursively pinned node")
	}

	assertPinTypes(t, ctx, api, []cidContainer{parent}, []cidContainer{}, []cidContainer{leaf})

	err = api.Pin().Add(ctx, path.FromCid(grandparent.Cid()), opt.Pin.Recursive(false))
	if err != nil {
		t.Fatal(err)
	}

	err = api.Pin().Add(ctx, path.FromCid(grandparent.Cid()))
	if err != nil {
		t.Fatal(err)
	}

	assertPinTypes(t, ctx, api, []cidContainer{grandparent, parent}, []cidContainer{}, []cidContainer{leaf})
}

func (tp *TestSuite) TestPinIsPinned(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	if err != nil {
		t.Fatal(err)
	}

	leaf, parent, grandparent := getThreeChainedNodes(t, ctx, api, "foofoo")

	assertNotPinned(t, ctx, api, newIPLDPath(t, grandparent.Cid()))
	assertNotPinned(t, ctx, api, newIPLDPath(t, parent.Cid()))
	assertNotPinned(t, ctx, api, newIPLDPath(t, leaf.Cid()))

	err = api.Pin().Add(ctx, newIPLDPath(t, parent.Cid()), opt.Pin.Recursive(true))
	if err != nil {
		t.Fatal(err)
	}

	assertNotPinned(t, ctx, api, newIPLDPath(t, grandparent.Cid()))
	assertIsPinned(t, ctx, api, newIPLDPath(t, parent.Cid()), "recursive")
	assertIsPinned(t, ctx, api, newIPLDPath(t, leaf.Cid()), "indirect")

	err = api.Pin().Add(ctx, newIPLDPath(t, grandparent.Cid()), opt.Pin.Recursive(false))
	if err != nil {
		t.Fatal(err)
	}

	assertIsPinned(t, ctx, api, newIPLDPath(t, grandparent.Cid()), "direct")
	assertIsPinned(t, ctx, api, newIPLDPath(t, parent.Cid()), "recursive")
	assertIsPinned(t, ctx, api, newIPLDPath(t, leaf.Cid()), "indirect")
}

type cidContainer interface {
	Cid() cid.Cid
}

type immutablePathCidContainer struct {
	path.ImmutablePath
}

func (i immutablePathCidContainer) Cid() cid.Cid {
	return i.RootCid()
}

func getThreeChainedNodes(t *testing.T, ctx context.Context, api iface.CoreAPI, leafData string) (cidContainer, cidContainer, cidContainer) {
	leaf, err := api.Unixfs().Add(ctx, strFile(leafData)())
	if err != nil {
		t.Fatal(err)
	}

	parent, err := ipldcbor.FromJSON(strings.NewReader(`{"lnk": {"/": "`+leaf.RootCid().String()+`"}}`), math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	grandparent, err := ipldcbor.FromJSON(strings.NewReader(`{"lnk": {"/": "`+parent.Cid().String()+`"}}`), math.MaxUint64, -1)
	if err != nil {
		t.Fatal(err)
	}

	if err := api.Dag().AddMany(ctx, []ipld.Node{parent, grandparent}); err != nil {
		t.Fatal(err)
	}

	return immutablePathCidContainer{leaf}, parent, grandparent
}

func assertPinTypes(t *testing.T, ctx context.Context, api iface.CoreAPI, recusive, direct, indirect []cidContainer) {
	assertPinLsAllConsistency(t, ctx, api)

	list, err := accPins(ctx, api, opt.Pin.Ls.Recursive())
	if err != nil {
		t.Fatal(err)
	}

	assertPinCids(t, list, recusive...)

	list, err = accPins(ctx, api, opt.Pin.Ls.Direct())
	if err != nil {
		t.Fatal(err)
	}

	assertPinCids(t, list, direct...)

	list, err = accPins(ctx, api, opt.Pin.Ls.Indirect())
	if err != nil {
		t.Fatal(err)
	}

	assertPinCids(t, list, indirect...)
}

// assertPinCids verifies that the pins match the expected cids
func assertPinCids(t *testing.T, pins []iface.Pin, cids ...cidContainer) {
	t.Helper()

	if expected, actual := len(cids), len(pins); expected != actual {
		t.Fatalf("expected pin list to have len %d, was %d", expected, actual)
	}

	cSet := cid.NewSet()
	for _, c := range cids {
		cSet.Add(c.Cid())
	}

	valid := true
	for _, p := range pins {
		c := p.Path().RootCid()
		if cSet.Has(c) {
			cSet.Remove(c)
		} else {
			valid = false
			break
		}
	}

	valid = valid && cSet.Len() == 0

	if !valid {
		pinStrs := make([]string, len(pins))
		for i, p := range pins {
			pinStrs[i] = p.Path().RootCid().String()
		}
		pathStrs := make([]string, len(cids))
		for i, c := range cids {
			pathStrs[i] = c.Cid().String()
		}
		t.Fatalf("expected: %s \nactual: %s", strings.Join(pathStrs, ", "), strings.Join(pinStrs, ", "))
	}
}

// assertPinLsAllConsistency verifies that listing all pins gives the same result as listing the pin types individually
func assertPinLsAllConsistency(t *testing.T, ctx context.Context, api iface.CoreAPI) {
	t.Helper()
	allPins, err := accPins(ctx, api)
	if err != nil {
		t.Fatal(err)
	}

	type pinTypeProps struct {
		*cid.Set
		opt.PinLsOption
	}

	all, recursive, direct, indirect := cid.NewSet(), cid.NewSet(), cid.NewSet(), cid.NewSet()
	typeMap := map[string]*pinTypeProps{
		"recursive": {recursive, opt.Pin.Ls.Recursive()},
		"direct":    {direct, opt.Pin.Ls.Direct()},
		"indirect":  {indirect, opt.Pin.Ls.Indirect()},
	}

	for _, p := range allPins {
		if !all.Visit(p.Path().RootCid()) {
			t.Fatalf("pin ls returned the same cid multiple times")
		}

		typeStr := p.Type()
		if typeSet, ok := typeMap[p.Type()]; ok {
			typeSet.Add(p.Path().RootCid())
		} else {
			t.Fatalf("unknown pin type: %s", typeStr)
		}
	}

	for typeStr, pinProps := range typeMap {
		pins, err := accPins(ctx, api, pinProps.PinLsOption)
		if err != nil {
			t.Fatal(err)
		}

		if expected, actual := len(pins), pinProps.Set.Len(); expected != actual {
			t.Fatalf("pin ls all has %d pins of type %s, but pin ls for the type has %d", expected, typeStr, actual)
		}

		for _, p := range pins {
			if pinType := p.Type(); pinType != typeStr {
				t.Fatalf("returned wrong pin type: expected %s, got %s", typeStr, pinType)
			}

			if c := p.Path().RootCid(); !pinProps.Has(c) {
				t.Fatalf("%s expected to be in pin ls all as type %s", c.String(), typeStr)
			}
		}
	}
}

func assertIsPinned(t *testing.T, ctx context.Context, api iface.CoreAPI, p path.Path, typeStr string) {
	t.Helper()
	withType, err := opt.Pin.IsPinned.Type(typeStr)
	if err != nil {
		t.Fatal("unhandled pin type")
	}

	whyPinned, pinned, err := api.Pin().IsPinned(ctx, p, withType)
	if err != nil {
		t.Fatal(err)
	}

	if !pinned {
		t.Fatalf("%s expected to be pinned with type %s", p, typeStr)
	}

	switch typeStr {
	case "recursive", "direct":
		if typeStr != whyPinned {
			t.Fatalf("reason for pinning expected to be %s for %s, got %s", typeStr, p, whyPinned)
		}
	case "indirect":
		if whyPinned == "" {
			t.Fatalf("expected to have a pin reason for %s", p)
		}
	}
}

func assertNotPinned(t *testing.T, ctx context.Context, api iface.CoreAPI, p path.Path) {
	t.Helper()

	_, pinned, err := api.Pin().IsPinned(ctx, p)
	if err != nil {
		t.Fatal(err)
	}

	if pinned {
		t.Fatalf("%s expected to not be pinned", p)
	}
}

func accPins(ctx context.Context, api iface.CoreAPI, opts ...opt.PinLsOption) ([]iface.Pin, error) {
	var err error
	pins := make(chan iface.Pin)
	go func() {
		err = api.Pin().Ls(ctx, pins, opts...)
	}()

	var results []iface.Pin
	for pin := range pins {
		results = append(results, pin)
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}
