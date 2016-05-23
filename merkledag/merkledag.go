// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"sync"

	blocks "github.com/ipfs/go-ipfs/blocks"
	key "github.com/ipfs/go-ipfs/blocks/key"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	logging "gx/ipfs/QmaDNZ4QMdBdku1YZWBysufYyoQt1negQGNav6PLYarbY8/go-log"
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

	NeedAltData() bool
}

// dagService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
// TODO: should cache Nodes that are in memory, and be
//       able to free some of them when vm pressure is high
type DefaultDagService struct {
	Blocks      *bserv.BlockService
	NodeToBlock NodeToBlock
}

func (n *DefaultDagService) NeedAltData() bool {
	return n.NodeToBlock.NeedAltData()
}

type NodeToBlock interface {
	CreateBlock(nd *Node) (blocks.Block, error)
	NeedAltData() bool
}

type nodeToBlock struct{}

func (nodeToBlock) CreateBlock(nd *Node) (blocks.Block, error) {
	return CreateBasicBlock(nd)
}

func CreateBasicBlock(nd *Node) (*blocks.BasicBlock, error) {
	d, err := nd.EncodeProtobuf(false)
	if err != nil {
		return nil, err
	}

	mh, err := nd.Multihash()
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithHash(d, mh)
}

func (nodeToBlock) NeedAltData() bool {
	return false
}

func NewDAGService(bs *bserv.BlockService) *DefaultDagService {
	return &DefaultDagService{bs, nodeToBlock{}}
}

// Add adds a node to the dagService, storing the block in the BlockService
func (n *DefaultDagService) Add(nd *Node) (key.Key, error) {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return "", fmt.Errorf("dagService is nil")
	}

	b, err := n.NodeToBlock.CreateBlock(nd)
	if err != nil {
		return "", err
	}

	return n.Blocks.AddBlock(b)
}

func (n *DefaultDagService) Batch() *Batch {
	return &Batch{ds: n, MaxSize: 8 * 1024 * 1024}
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *DefaultDagService) Get(ctx context.Context, k key.Key) (*Node, error) {
	if k == "" {
		return nil, ErrNotFound
	}
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

	res, err := DecodeProtobuf(b.Data())
	if err != nil {
		return nil, fmt.Errorf("Failed to decode Protocol Buffers: %v", err)
	}
	return res, nil
}

func (n *DefaultDagService) Remove(nd *Node) error {
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

func (ds *DefaultDagService) GetMany(ctx context.Context, keys []key.Key) <-chan *NodeOption {
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
				nd, err := DecodeProtobuf(b.Data())
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
	for i := range keys {
		promises[i] = newNodePromise(ctx)
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
					for _, p := range promises {
						p.Fail(ErrNotFound)
					}
					return
				}

				if opt.Err != nil {
					for _, p := range promises {
						p.Fail(opt.Err)
					}
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
					promises[i].Send(nd)
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

func newNodePromise(ctx context.Context) NodeGetter {
	return &nodePromise{
		recv: make(chan *Node, 1),
		ctx:  ctx,
		err:  make(chan error, 1),
	}
}

type nodePromise struct {
	cache *Node
	clk   sync.Mutex
	recv  chan *Node
	ctx   context.Context
	err   chan error
}

// NodeGetter provides a promise like interface for a dag Node
// the first call to Get will block until the Node is received
// from its internal channels, subsequent calls will return the
// cached node.
type NodeGetter interface {
	Get(context.Context) (*Node, error)
	Fail(err error)
	Send(*Node)
}

func (np *nodePromise) Fail(err error) {
	np.clk.Lock()
	v := np.cache
	np.clk.Unlock()

	// if promise has a value, don't fail it
	if v != nil {
		return
	}

	np.err <- err
}

func (np *nodePromise) Send(nd *Node) {
	var already bool
	np.clk.Lock()
	if np.cache != nil {
		already = true
	}
	np.cache = nd
	np.clk.Unlock()

	if already {
		panic("sending twice to the same promise is an error!")
	}

	np.recv <- nd
}

func (np *nodePromise) Get(ctx context.Context) (*Node, error) {
	np.clk.Lock()
	c := np.cache
	np.clk.Unlock()
	if c != nil {
		return c, nil
	}

	select {
	case nd := <-np.recv:
		return nd, nil
	case <-np.ctx.Done():
		return nil, np.ctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-np.err:
		return nil, err
	}
}

type Batch struct {
	ds *DefaultDagService

	blocks  []blocks.Block
	size    int
	MaxSize int
}

func (t *Batch) Add(nd *Node) (key.Key, error) {
	b, err := t.ds.NodeToBlock.CreateBlock(nd)
	if err != nil {
		return "", err
	}

	t.blocks = append(t.blocks, b)
	t.size += len(b.Data())
	if t.size > t.MaxSize {
		return b.Key(), t.Commit()
	}
	return b.Key(), nil
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
