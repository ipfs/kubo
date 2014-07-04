package merkledag

import (
	"fmt"
	// mh "github.com/jbenet/go-multihash"
	"testing"
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
	}

	printn("beep", n1)
	printn("boop", n2)
	printn("beep boop", n3)
}
