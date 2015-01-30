package traverse

import (
	"bytes"
	"fmt"
	"testing"

	mdag "github.com/jbenet/go-ipfs/struct/merkledag"
)

func TestDFSPreNoSkip(t *testing.T) {
	opts := Options{Order: DFSPre}

	testWalkOutputs(t, newFan(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aab
1 /a/ab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestDFSPreSkip(t *testing.T) {
	opts := Options{Order: DFSPre, SkipDuplicates: true}

	testWalkOutputs(t, newFan(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aab
1 /a/ab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestDFSPostNoSkip(t *testing.T) {
	opts := Options{Order: DFSPost}

	testWalkOutputs(t, newFan(t), opts, []byte(`
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
0 /a
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
2 /a/aa/aaa
2 /a/aa/aab
1 /a/aa
2 /a/ab/aba
2 /a/ab/abb
1 /a/ab
0 /a
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))
}

func TestDFSPostSkip(t *testing.T) {
	opts := Options{Order: DFSPost, SkipDuplicates: true}

	testWalkOutputs(t, newFan(t), opts, []byte(`
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
0 /a
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
2 /a/aa/aaa
2 /a/aa/aab
1 /a/aa
2 /a/ab/aba
2 /a/ab/abb
1 /a/ab
0 /a
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
4 /a/aa/aaa/aaaa/aaaaa
3 /a/aa/aaa/aaaa
2 /a/aa/aaa
1 /a/aa
0 /a
`))
}

func TestBFSNoSkip(t *testing.T) {
	opts := Options{Order: BFS}

	testWalkOutputs(t, newFan(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
2 /a/aa/aaa
2 /a/aa/aab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/aa
2 /a/aa/aaa
2 /a/aa/aaa
2 /a/aa/aaa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func TestBFSSkip(t *testing.T) {
	opts := Options{Order: BFS, SkipDuplicates: true}

	testWalkOutputs(t, newFan(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
1 /a/ac
1 /a/ad
`))

	testWalkOutputs(t, newLinkedList(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))

	testWalkOutputs(t, newBinaryTree(t), opts, []byte(`
0 /a
1 /a/aa
1 /a/ab
2 /a/aa/aaa
2 /a/aa/aab
2 /a/ab/aba
2 /a/ab/abb
`))

	testWalkOutputs(t, newBinaryDAG(t), opts, []byte(`
0 /a
1 /a/aa
2 /a/aa/aaa
3 /a/aa/aaa/aaaa
4 /a/aa/aaa/aaaa/aaaaa
`))
}

func testWalkOutputs(t *testing.T, root *mdag.Node, opts Options, expect []byte) {
	expect = bytes.TrimLeft(expect, "\n")

	var buf bytes.Buffer
	walk := func(current State) error {
		s := fmt.Sprintf("%d %s\n", current.Depth, current.Node.Data)
		t.Logf("walk: %s", s)
		buf.Write([]byte(s))
		return nil
	}

	opts.Func = walk
	if err := Traverse(root, opts); err != nil {
		t.Error(err)
		return
	}

	actual := buf.Bytes()
	if !bytes.Equal(actual, expect) {
		t.Error("error: outputs differ")
		t.Logf("expect:\n%s", expect)
		t.Logf("actual:\n%s", actual)
	} else {
		t.Logf("expect matches actual:\n%s", expect)
	}
}

func newFan(t *testing.T) *mdag.Node {
	a := &mdag.Node{Data: []byte("/a")}
	addChild(t, a, "aa")
	addChild(t, a, "ab")
	addChild(t, a, "ac")
	addChild(t, a, "ad")
	return a
}

func newLinkedList(t *testing.T) *mdag.Node {
	a := &mdag.Node{Data: []byte("/a")}
	aa := addChild(t, a, "aa")
	aaa := addChild(t, aa, "aaa")
	aaaa := addChild(t, aaa, "aaaa")
	addChild(t, aaaa, "aaaaa")
	return a
}

func newBinaryTree(t *testing.T) *mdag.Node {
	a := &mdag.Node{Data: []byte("/a")}
	aa := addChild(t, a, "aa")
	ab := addChild(t, a, "ab")
	addChild(t, aa, "aaa")
	addChild(t, aa, "aab")
	addChild(t, ab, "aba")
	addChild(t, ab, "abb")
	return a
}

func newBinaryDAG(t *testing.T) *mdag.Node {
	a := &mdag.Node{Data: []byte("/a")}
	aa := addChild(t, a, "aa")
	aaa := addChild(t, aa, "aaa")
	aaaa := addChild(t, aaa, "aaaa")
	aaaaa := addChild(t, aaaa, "aaaaa")
	addLink(t, a, aa)
	addLink(t, aa, aaa)
	addLink(t, aaa, aaaa)
	addLink(t, aaaa, aaaaa)
	return a
}

func addLink(t *testing.T, a, b *mdag.Node) {
	to := string(a.Data) + "2" + string(b.Data)
	if err := a.AddNodeLink(to, b); err != nil {
		t.Error(err)
	}
}

func addChild(t *testing.T, a *mdag.Node, name string) *mdag.Node {
	c := &mdag.Node{Data: []byte(string(a.Data) + "/" + name)}
	addLink(t, a, c)
	return c
}
