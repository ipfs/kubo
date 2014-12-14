package merkledag_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	blockservice "github.com/jbenet/go-ipfs/blockservice"
	imp "github.com/jbenet/go-ipfs/importer"
	chunk "github.com/jbenet/go-ipfs/importer/chunk"
	. "github.com/jbenet/go-ipfs/merkledag"
	uio "github.com/jbenet/go-ipfs/unixfs/io"
	u "github.com/jbenet/go-ipfs/util"
)

func TestNode(t *testing.T) {

	n1 := &Node{Data: []byte("beep")}
	n2 := &Node{Data: []byte("boop")}
	n3 := &Node{Data: []byte("beep boop")}
	if err := n3.AddNodeLink("beep-link", n1); err != nil {
		t.Error(err)
	}
	if err := n3.AddNodeLink("boop-link", n2); err != nil {
		t.Error(err)
	}

	printn := func(name string, n *Node) {
		fmt.Println(">", name)
		fmt.Println("data:", string(n.Data))

		fmt.Println("links:")
		for _, l := range n.Links {
			fmt.Println("-", l.Name, l.Size, l.Hash)
		}

		e, err := n.Encoded(false)
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("encoded:", e)
		}

		h, err := n.Multihash()
		if err != nil {
			t.Error(err)
		} else {
			fmt.Println("hash:", h)
		}

		k, err := n.Key()
		if err != nil {
			t.Error(err)
		} else if k != u.Key(h) {
			t.Error("Key is not equivalent to multihash")
		} else {
			fmt.Println("key: ", k)
		}
	}

	printn("beep", n1)
	printn("boop", n2)
	printn("beep boop", n3)
}

func makeTestDag(t *testing.T) *Node {
	read := io.LimitReader(u.NewTimeSeededRand(), 1024*32)
	spl := &chunk.SizeSplitter{512}
	root, err := imp.NewDagFromReaderWithSplitter(read, spl)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

type devZero struct{}

func (_ devZero) Read(b []byte) (int, error) {
	for i, _ := range b {
		b[i] = 0
	}
	return len(b), nil
}

func makeZeroDag(t *testing.T) *Node {
	read := io.LimitReader(devZero{}, 1024*32)
	spl := &chunk.SizeSplitter{512}
	root, err := imp.NewDagFromReaderWithSplitter(read, spl)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBatchFetch(t *testing.T) {
	var dagservs []DAGService
	for _, bsi := range blockservice.Mocks(t, 5) {
		dagservs = append(dagservs, NewDAGService(bsi))
	}
	t.Log("finished setup.")

	root := makeTestDag(t)
	read, err := uio.NewDagReader(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = dagservs[0].AddRecursive(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Added file to first node.")

	k, err := root.Key()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	for i := 1; i < len(dagservs); i++ {
		go func(i int) {
			first, err := dagservs[i].Get(k)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("Got first node back.")

			read, err := uio.NewDagReader(first, dagservs[i])
			if err != nil {
				t.Fatal(err)
			}
			datagot, err := ioutil.ReadAll(read)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(datagot, expected) {
				t.Fatal("Got bad data back!")
			}
			done <- struct{}{}
		}(i)
	}

	for i := 1; i < len(dagservs); i++ {
		<-done
	}
}

func TestBatchFetchDupBlock(t *testing.T) {
	var dagservs []DAGService
	for _, bsi := range blockservice.Mocks(t, 5) {
		dagservs = append(dagservs, NewDAGService(bsi))
	}
	t.Log("finished setup.")

	root := makeZeroDag(t)
	read, err := uio.NewDagReader(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := ioutil.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}

	err = dagservs[0].AddRecursive(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("Added file to first node.")

	k, err := root.Key()
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	for i := 1; i < len(dagservs); i++ {
		go func(i int) {
			first, err := dagservs[i].Get(k)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Println("Got first node back.")

			read, err := uio.NewDagReader(first, dagservs[i])
			if err != nil {
				t.Fatal(err)
			}
			datagot, err := ioutil.ReadAll(read)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(datagot, expected) {
				t.Fatal("Got bad data back!")
			}
			done <- struct{}{}
		}(i)
	}

	for i := 1; i < len(dagservs); i++ {
		<-done
	}
}
