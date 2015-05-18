// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"sync"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	blocks "github.com/ipfs/go-ipfs/blocks"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("merkledag")
var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(*Node) (u.Key, error)
	AddRecursive(*Node) error
	Get(context.Context, u.Key) (*Node, error)
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
func (n *dagService) Get(ctx context.Context, k u.Key) (*Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}

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

			nd, err := lnk.GetNode(ctx, serv)
			if err != nil {
				log.Debug(err)
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

// GetNodes returns an array of 'NodeGetter' promises, with each corresponding
// to the key with the same index as the passed in keys
func (ds *dagService) GetNodes(ctx context.Context, keys []u.Key) []NodeGetter {

	// Early out if no work to do
	if len(keys) == 0 {
		return nil
	}

	promises := make([]NodeGetter, len(keys))
	sendChans := make([]chan<- *Node, len(keys))
	for i := range keys {
		promises[i], sendChans[i] = newNodePromise(ctx)
	}

	dedupedKeys := dedupeKeys(keys)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		blkchan := ds.Blocks.GetBlocks(ctx, dedupedKeys)

		for count := 0; count < len(keys); {
			select {
			case blk, ok := <-blkchan:
				if !ok {
					return
				}

				nd, err := Decoded(blk.Data)
				if err != nil {
					// NB: can happen with improperly formatted input data
					log.Debug("Got back bad block!")
					return
				}
				is := FindLinks(keys, blk.Key(), 0)
				for _, i := range is {
					count++
					sendChans[i] <- nd
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return promises
}

// Remove duplicates from a list of keys
func dedupeKeys(ks []u.Key) []u.Key {
	kmap := make(map[u.Key]struct{})
	var out []u.Key
	for _, k := range ks {
		if _, ok := kmap[k]; !ok {
			kmap[k] = struct{}{}
			out = append(out, k)
		}
	}
	return out
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

// NodeGetter provides a promise like interface for a dag Node
// the first call to Get will block until the Node is received
// from its internal channels, subsequent calls will return the
// cached node.
type NodeGetter interface {
	Get(context.Context) (*Node, error)
}

func (np *nodePromise) Get(ctx context.Context) (*Node, error) {
	if np.cache != nil {
		return np.cache, nil
	}

	select {
	case blk := <-np.recv:
		np.cache = blk
	case <-np.ctx.Done():
		return nil, np.ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return np.cache, nil
}
