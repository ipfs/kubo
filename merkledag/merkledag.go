// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"sync"

	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	logging "gx/ipfs/Qmazh5oNUVsDZTs2g59rq8aYQqwpss8tcUWQzor5sCCEuH/go-log"
)

var log = logging.Logger("merkledag")
var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(*Node) (key.Key, error)
	Get(context.Context, key.Key) (*Node, error)
	Remove(*Node) error

	// GetDAG returns, in order, all the single leve child
	// nodes of the passed in node.
	GetMany(context.Context, []key.Key) <-chan *NodeOption

	Batch() *Batch
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
func (n *dagService) Add(nd *Node) (key.Key, error) {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return "", fmt.Errorf("dagService is nil")
	}

	d, err := nd.EncodeProtobuf(false)
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

func (n *dagService) Batch() *Batch {
	return &Batch{ds: n, MaxSize: 8 * 1024 * 1024}
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(ctx context.Context, k key.Key) (*Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := n.Blocks.GetBlock(ctx, k)
	if err != nil {
		if err == bserv.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("Failed to get block for %s: %v", k.B58String(), err)
	}

	res, err := DecodeProtobuf(b.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to decode Protocol Buffers: %v", err)
	}
	return res, nil
}

func (n *dagService) Remove(nd *Node) error {
	k, err := nd.Key()
	if err != nil {
		return err
	}
	return n.Blocks.DeleteBlock(k)
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root *Node, serv DAGService) error {
	return EnumerateChildrenAsync(ctx, serv, root, key.NewKeySet())
}

// FindLinks searches this nodes links for the given key,
// returns the indexes of any links pointing to it
func FindLinks(links []key.Key, k key.Key, start int) []int {
	var out []int
	for i, lnk_k := range links[start:] {
		if k == lnk_k {
			out = append(out, i+start)
		}
	}
	return out
}

type NodeOption struct {
	Node *Node
	Err  error
}

func (ds *dagService) GetMany(ctx context.Context, keys []key.Key) <-chan *NodeOption {
	out := make(chan *NodeOption, len(keys))
	blocks := ds.Blocks.GetBlocks(ctx, keys)
	var count int

	go func() {
		defer close(out)
		for {
			select {
			case b, ok := <-blocks:
				if !ok {
					if count != len(keys) {
						out <- &NodeOption{Err: fmt.Errorf("failed to fetch all nodes")}
					}
					return
				}
				nd, err := DecodeProtobuf(b.Data)
				if err != nil {
					out <- &NodeOption{Err: err}
					return
				}

				// buffered, no need to select
				out <- &NodeOption{Node: nd}
				count++

			case <-ctx.Done():
				out <- &NodeOption{Err: ctx.Err()}
				return
			}
		}
	}()
	return out
}

// GetDAG will fill out all of the links of the given Node.
// It returns a channel of nodes, which the caller can receive
// all the child nodes of 'root' on, in proper order.
func GetDAG(ctx context.Context, ds DAGService, root *Node) []NodeGetter {
	var keys []key.Key
	for _, lnk := range root.Links {
		keys = append(keys, key.Key(lnk.Hash))
	}

	return GetNodes(ctx, ds, keys)
}

// GetNodes returns an array of 'NodeGetter' promises, with each corresponding
// to the key with the same index as the passed in keys
func GetNodes(ctx context.Context, ds DAGService, keys []key.Key) []NodeGetter {

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

		nodechan := ds.GetMany(ctx, dedupedKeys)

		for count := 0; count < len(keys); {
			select {
			case opt, ok := <-nodechan:
				if !ok {
					return
				}

				if opt.Err != nil {
					log.Error("error fetching: ", opt.Err)
					return
				}

				nd := opt.Node

				k, err := nd.Key()
				if err != nil {
					log.Error("Failed to get node key: ", err)
					continue
				}

				is := FindLinks(keys, k, 0)
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
func dedupeKeys(ks []key.Key) []key.Key {
	kmap := make(map[key.Key]struct{})
	var out []key.Key
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

type Batch struct {
	ds *dagService

	blocks  []*blocks.Block
	size    int
	MaxSize int
}

func (t *Batch) Add(nd *Node) (key.Key, error) {
	d, err := nd.EncodeProtobuf(false)
	if err != nil {
		return "", err
	}

	b := new(blocks.Block)
	b.Data = d
	b.Multihash, err = nd.Multihash()
	if err != nil {
		return "", err
	}

	k := key.Key(b.Multihash)

	t.blocks = append(t.blocks, b)
	t.size += len(b.Data)
	if t.size > t.MaxSize {
		return k, t.Commit()
	}
	return k, nil
}

func (t *Batch) Commit() error {
	_, err := t.ds.Blocks.AddBlocks(t.blocks)
	t.blocks = nil
	t.size = 0
	return err
}

// EnumerateChildren will walk the dag below the given root node and add all
// unseen children to the passed in set.
// TODO: parallelize to avoid disk latency perf hits?
func EnumerateChildren(ctx context.Context, ds DAGService, root *Node, set key.KeySet) error {
	for _, lnk := range root.Links {
		k := key.Key(lnk.Hash)
		if !set.Has(k) {
			set.Add(k)
			child, err := ds.Get(ctx, k)
			if err != nil {
				return err
			}
			err = EnumerateChildren(ctx, ds, child, set)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func EnumerateChildrenAsync(ctx context.Context, ds DAGService, root *Node, set key.KeySet) error {
	toprocess := make(chan []key.Key, 8)
	nodes := make(chan *NodeOption, 8)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(toprocess)

	go fetchNodes(ctx, ds, toprocess, nodes)

	nodes <- &NodeOption{Node: root}
	live := 1

	for {
		select {
		case opt, ok := <-nodes:
			if !ok {
				return nil
			}

			if opt.Err != nil {
				return opt.Err
			}

			nd := opt.Node

			// a node has been fetched
			live--

			var keys []key.Key
			for _, lnk := range nd.Links {
				k := key.Key(lnk.Hash)
				if !set.Has(k) {
					set.Add(k)
					live++
					keys = append(keys, k)
				}
			}

			if live == 0 {
				return nil
			}

			if len(keys) > 0 {
				select {
				case toprocess <- keys:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func fetchNodes(ctx context.Context, ds DAGService, in <-chan []key.Key, out chan<- *NodeOption) {
	var wg sync.WaitGroup
	defer func() {
		// wait for all 'get' calls to complete so we don't accidentally send
		// on a closed channel
		wg.Wait()
		close(out)
	}()

	get := func(ks []key.Key) {
		defer wg.Done()
		nodes := ds.GetMany(ctx, ks)
		for opt := range nodes {
			select {
			case out <- opt:
			case <-ctx.Done():
				return
			}
		}
	}

	for ks := range in {
		wg.Add(1)
		go get(ks)
	}
}
