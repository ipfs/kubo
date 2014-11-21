// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"sync"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	blocks "github.com/jbenet/go-ipfs/blocks"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("merkledag")
var ErrNotFound = fmt.Errorf("merkledag: not found")

// NodeMap maps u.Keys to Nodes.
// We cannot use []byte/Multihash for keys :(
// so have to convert Multihash bytes to string (u.Key)
type NodeMap map[u.Key]*Node

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(*Node) (u.Key, error)
	AddRecursive(*Node) error
	Get(u.Key) (*Node, error)
	Remove(*Node) error
	BatchFetch(context.Context, *Node) <-chan *Node
}

func NewDAGService(bs *bserv.BlockService) DAGService {
	return &dagService{bs}
}

// Node represents a node in the IPFS Merkle DAG.
// nodes have opaque data and a set of navigable links.
type Node struct {
	Links []*Link
	Data  []byte

	// cache encoded/marshaled value
	encoded []byte

	cached mh.Multihash
}

// Link represents an IPFS Merkle DAG Link between Nodes.
type Link struct {
	// utf string name. should be unique per object
	Name string // utf8

	// cumulative size of target object
	Size uint64

	// multihash of the target object
	Hash mh.Multihash

	// a ptr to the actual node for graph manipulation
	Node *Node
}

// MakeLink creates a link to the given node
func MakeLink(n *Node) (*Link, error) {
	s, err := n.Size()
	if err != nil {
		return nil, err
	}

	h, err := n.Multihash()
	if err != nil {
		return nil, err
	}
	return &Link{
		Size: s,
		Hash: h,
	}, nil
}

// GetNode returns the MDAG Node that this link points to
func (l *Link) GetNode(serv DAGService) (*Node, error) {
	if l.Node != nil {
		return l.Node, nil
	}

	return serv.Get(u.Key(l.Hash))
}

// AddNodeLink adds a link to another node.
func (n *Node) AddNodeLink(name string, that *Node) error {
	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}
	lnk.Name = name
	lnk.Node = that

	n.Links = append(n.Links, lnk)
	return nil
}

// AddNodeLink adds a link to another node. without keeping a reference to
// the child node
func (n *Node) AddNodeLinkClean(name string, that *Node) error {
	lnk, err := MakeLink(that)
	if err != nil {
		return err
	}
	lnk.Name = name

	n.Links = append(n.Links, lnk)
	return nil
}

// Remove a link on this node by the given name
func (n *Node) RemoveNodeLink(name string) error {
	for i, l := range n.Links {
		if l.Name == name {
			n.Links = append(n.Links[:i], n.Links[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

// Copy returns a copy of the node.
// NOTE: does not make copies of Node objects in the links.
func (n *Node) Copy() *Node {
	nnode := new(Node)
	nnode.Data = make([]byte, len(n.Data))
	copy(nnode.Data, n.Data)

	nnode.Links = make([]*Link, len(n.Links))
	copy(nnode.Links, n.Links)
	return nnode
}

// Size returns the total size of the data addressed by node,
// including the total sizes of references.
func (n *Node) Size() (uint64, error) {
	b, err := n.Encoded(false)
	if err != nil {
		return 0, err
	}

	s := uint64(len(b))
	for _, l := range n.Links {
		s += l.Size
	}
	return s, nil
}

// Multihash hashes the encoded data of this node.
func (n *Node) Multihash() (mh.Multihash, error) {
	// Note: Encoded generates the hash and puts it in n.cached.
	_, err := n.Encoded(false)
	if err != nil {
		return nil, err
	}

	return n.cached, nil
}

// Key returns the Multihash as a key, for maps.
func (n *Node) Key() (u.Key, error) {
	h, err := n.Multihash()
	return u.Key(h), err
}

// dagService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
// TODO: should cache Nodes that are in memory, and be
//       able to free some of them when vm pressure is high
type dagService struct {
	Blocks *bserv.BlockService
}

// Add adds a node to the dagService, storing the block in the BlockService
func (n *dagService) Add(nd *Node) (u.Key, error) {
	k, _ := nd.Key()
	log.Debugf("DagService Add [%s]", k)
	if n == nil {
		return "", fmt.Errorf("dagService is nil")
	}

	d, err := nd.Encoded(false)
	if err != nil {
		return "", err
	}

	b := new(blocks.Block)
	b.Data = d
	b.Multihash, err = nd.Multihash()
	if err != nil {
		return "", err
	}

	return n.Blocks.AddBlock(b)
}

// AddRecursive adds the given node and all child nodes to the BlockService
func (n *dagService) AddRecursive(nd *Node) error {
	_, err := n.Add(nd)
	if err != nil {
		log.Info("AddRecursive Error: %s\n", err)
		return err
	}

	for _, link := range nd.Links {
		if link.Node != nil {
			err := n.AddRecursive(link.Node)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(k u.Key) (*Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}

	ctx, _ := context.WithTimeout(context.TODO(), time.Second*5)
	b, err := n.Blocks.GetBlock(ctx, k)
	if err != nil {
		return nil, err
	}

	return Decoded(b.Data)
}

// Remove deletes the given node and all of its children from the BlockService
func (n *dagService) Remove(nd *Node) error {
	for _, l := range nd.Links {
		if l.Node != nil {
			n.Remove(l.Node)
		}
	}
	k, err := nd.Key()
	if err != nil {
		return err
	}
	return n.Blocks.DeleteBlock(k)
}

// FetchGraph asynchronously fetches all nodes that are children of the given
// node, and returns a channel that may be waited upon for the fetch to complete
func FetchGraph(ctx context.Context, root *Node, serv DAGService) chan struct{} {
	log.Warning("Untested.")
	var wg sync.WaitGroup
	done := make(chan struct{})

	for _, l := range root.Links {
		wg.Add(1)
		go func(lnk *Link) {

			// Signal child is done on way out
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			}

			nd, err := lnk.GetNode(serv)
			if err != nil {
				log.Error(err)
				return
			}

			// Wait for children to finish
			<-FetchGraph(ctx, nd, serv)
		}(l)
	}

	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	return done
}

// BatchFetch will fill out all of the links of the given Node.
// It returns a channel of indicies, which will be returned in order
// from 0 to len(root.Links) - 1, signalling that the link specified by
// the index has been filled out.
func (ds *dagService) BatchFetch(ctx context.Context, root *Node) <-chan *Node {
	sig := make(chan *Node)
	go func() {
		var keys []u.Key
		nodes := make([]*Node, len(root.Links))

		//
		next := 0
		seen := make(map[int]struct{})
		//

		for _, lnk := range root.Links {
			keys = append(keys, u.Key(lnk.Hash))
		}

		blkchan := ds.Blocks.GetBlocks(ctx, keys)

		for blk := range blkchan {
			for i, lnk := range root.Links {
				if u.Key(lnk.Hash) != blk.Key() {
					continue
				}

				//
				seen[i] = struct{}{}
				//

				nd, err := Decoded(blk.Data)
				if err != nil {
					log.Error("Got back bad block!")
					break
				}
				nodes[i] = nd

				if next == i {
					sig <- nd
					next++
					for ; next < len(nodes) && nodes[next] != nil; next++ {
						sig <- nodes[next]
					}
				}
			}
		}
		close(sig)
	}()

	return sig
}
