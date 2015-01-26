// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"sync"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	blocks "github.com/jbenet/go-ipfs/blocks"
	bserv "github.com/jbenet/go-ipfs/blockservice"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("merkledag")
var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(*Node) (u.Key, error)
	AddRecursive(*Node) error
	Get(u.Key) (*Node, error)
	Remove(*Node) error

	// GetDAG returns, in order, all the single leve child
	// nodes of the passed in node.
	GetDAG(context.Context, *Node) []NodeGetter
	GetNodes(context.Context, []u.Key) []NodeGetter
}

func NewDAGService(bs *bserv.BlockService) DAGService {
	return &dagService{bs}
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
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
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

	ctx, _ := context.WithTimeout(context.TODO(), time.Minute)
	// we shouldn't use an arbitrary timeout here.
	// since Get doesnt take in a context yet, we give a large upper bound.
	// think of an http request. we want it to go on as long as the client requests it.

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

// FindLinks searches this nodes links for the given key,
// returns the indexes of any links pointing to it
func FindLinks(links []u.Key, k u.Key, start int) []int {
	var out []int
	for i, lnk_k := range links[start:] {
		if k == lnk_k {
			out = append(out, i+start)
		}
	}
	return out
}

// GetDAG will fill out all of the links of the given Node.
// It returns a channel of nodes, which the caller can receive
// all the child nodes of 'root' on, in proper order.
func (ds *dagService) GetDAG(ctx context.Context, root *Node) []NodeGetter {
	var keys []u.Key
	for _, lnk := range root.Links {
		keys = append(keys, u.Key(lnk.Hash))
	}

	return ds.GetNodes(ctx, keys)
}

func (ds *dagService) GetNodes(ctx context.Context, keys []u.Key) []NodeGetter {
	promises := make([]NodeGetter, len(keys))
	sendChans := make([]chan<- *Node, len(keys))
	for i, _ := range keys {
		promises[i], sendChans[i] = newNodePromise(ctx)
	}

	go func() {
		blkchan := ds.Blocks.GetBlocks(ctx, keys)

		for {
			select {
			case blk, ok := <-blkchan:
				if !ok {
					return
				}

				nd, err := Decoded(blk.Data)
				if err != nil {
					// NB: can happen with improperly formatted input data
					log.Error("Got back bad block!")
					return
				}
				is := FindLinks(keys, blk.Key(), 0)
				for _, i := range is {
					sendChans[i] <- nd
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return promises
}

func newNodePromise(ctx context.Context) (NodeGetter, chan<- *Node) {
	ch := make(chan *Node, 1)
	return &nodePromise{
		recv: ch,
		ctx:  ctx,
	}, ch
}

type nodePromise struct {
	cache *Node
	recv  <-chan *Node
	ctx   context.Context
}

type NodeGetter interface {
	Get() (*Node, error)
}

func (np *nodePromise) Get() (*Node, error) {
	if np.cache != nil {
		return np.cache, nil
	}

	select {
	case blk := <-np.recv:
		np.cache = blk
	case <-np.ctx.Done():
		return nil, np.ctx.Err()
	}
	return np.cache, nil
}
