// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	"fmt"
	"strings"
	"sync"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	logging "gx/ipfs/QmSpJByNKFX1sCsHBEp3R73FL4NF6FnQTEGyNAXHm2GS52/go-log"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

var log = logging.Logger("merkledag")
var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(*Node) (*cid.Cid, error)
	Get(context.Context, *cid.Cid) (*Node, error)
	Remove(*Node) error

	// GetDAG returns, in order, all the single leve child
	// nodes of the passed in node.
	GetMany(context.Context, []*cid.Cid) <-chan *NodeOption

	Batch() *Batch

	LinkService
}

type LinkService interface {
	// Return all links for a node, may be more effect than
	// calling Get
	GetLinks(context.Context, *cid.Cid) ([]*Link, error)

	GetOfflineLinkService() LinkService
}

func NewDAGService(bs *bserv.BlockService) *dagService {
	return &dagService{Blocks: bs}
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
func (n *dagService) Add(nd *Node) (*cid.Cid, error) {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return nil, fmt.Errorf("dagService is nil")
	}

	return n.Blocks.AddObject(nd)
}

func (n *dagService) Batch() *Batch {
	return &Batch{ds: n, MaxSize: 8 * 1024 * 1024}
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(ctx context.Context, c *cid.Cid) (*Node, error) {
	if n == nil {
		return nil, fmt.Errorf("dagService is nil")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	b, err := n.Blocks.GetBlock(ctx, c)
	if err != nil {
		if err == bserv.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("Failed to get block for %s: %v", c, err)
	}

	var res *Node
	switch c.Type() {
	case cid.Protobuf:
		out, err := DecodeProtobuf(b.RawData())
		if err != nil {
			if strings.Contains(err.Error(), "Unmarshal failed") {
				return nil, fmt.Errorf("The block referred to by '%s' was not a valid merkledag node", c)
			}
			return nil, fmt.Errorf("Failed to decode Protocol Buffers: %v", err)
		}
		res = out
	default:
		return nil, fmt.Errorf("unrecognized formatting type")
	}

	res.cached = c

	return res, nil
}

func (n *dagService) GetLinks(ctx context.Context, c *cid.Cid) ([]*Link, error) {
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links, nil
}

func (n *dagService) GetOfflineLinkService() LinkService {
	if n.Blocks.Exchange.IsOnline() {
		bsrv := bserv.New(n.Blocks.Blockstore, offline.Exchange(n.Blocks.Blockstore))
		return NewDAGService(bsrv)
	} else {
		return n
	}
}

func (n *dagService) Remove(nd *Node) error {
	return n.Blocks.DeleteObject(nd)
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root *Node, serv DAGService) error {
	return EnumerateChildrenAsync(ctx, serv, root, cid.NewSet().Visit)
}

// FindLinks searches this nodes links for the given key,
// returns the indexes of any links pointing to it
func FindLinks(links []*cid.Cid, c *cid.Cid, start int) []int {
	var out []int
	for i, lnk_c := range links[start:] {
		if c.Equals(lnk_c) {
			out = append(out, i+start)
		}
	}
	return out
}

type NodeOption struct {
	Node *Node
	Err  error
}

// TODO: this is a mid-term hack to get around the fact that blocks don't
// have full CIDs and potentially (though we don't know of any such scenario)
// may have the same block with multiple different encodings.
// We have discussed the possiblity of using CIDs as datastore keys
// in the future. This would be a much larger changeset than i want to make
// right now.
func cidsToKeyMapping(cids []*cid.Cid) map[key.Key]*cid.Cid {
	mapping := make(map[key.Key]*cid.Cid)
	for _, c := range cids {
		mapping[key.Key(c.Hash())] = c
	}
	return mapping
}

func (ds *dagService) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *NodeOption {
	out := make(chan *NodeOption, len(keys))
	blocks := ds.Blocks.GetBlocks(ctx, keys)
	var count int

	mapping := cidsToKeyMapping(keys)

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

				c := mapping[b.Key()]

				var nd *Node
				switch c.Type() {
				case cid.Protobuf:
					decnd, err := DecodeProtobuf(b.RawData())
					if err != nil {
						out <- &NodeOption{Err: err}
						return
					}
					decnd.cached = cid.NewCidV0(b.Multihash())
					nd = decnd
				default:
					out <- &NodeOption{Err: fmt.Errorf("unrecognized object type: %s", c.Type())}
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
	var cids []*cid.Cid
	for _, lnk := range root.Links {
		cids = append(cids, cid.NewCidV0(lnk.Hash))
	}

	return GetNodes(ctx, ds, cids)
}

// GetNodes returns an array of 'NodeGetter' promises, with each corresponding
// to the key with the same index as the passed in keys
func GetNodes(ctx context.Context, ds DAGService, keys []*cid.Cid) []NodeGetter {

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
				is := FindLinks(keys, nd.Cid(), 0)
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
func dedupeKeys(cids []*cid.Cid) []*cid.Cid {
	set := cid.NewSet()
	for _, c := range cids {
		set.Add(c)
	}
	return set.Keys()
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
	ds *dagService

	objects []bserv.Object
	size    int
	MaxSize int
}

func (t *Batch) Add(nd *Node) (*cid.Cid, error) {
	d, err := nd.EncodeProtobuf(false)
	if err != nil {
		return nil, err
	}

	t.objects = append(t.objects, nd)
	t.size += len(d)
	if t.size > t.MaxSize {
		return nd.Cid(), t.Commit()
	}
	return nd.Cid(), nil
}

func (t *Batch) Commit() error {
	_, err := t.ds.Blocks.AddObjects(t.objects)
	t.objects = nil
	t.size = 0
	return err
}

func legacyCidFromLink(lnk *Link) *cid.Cid {
	return cid.NewCidV0(lnk.Hash)
}

// EnumerateChildren will walk the dag below the given root node and add all
// unseen children to the passed in set.
// TODO: parallelize to avoid disk latency perf hits?
func EnumerateChildren(ctx context.Context, ds LinkService, links []*Link, visit func(*cid.Cid) bool, bestEffort bool) error {
	for _, lnk := range links {
		c := legacyCidFromLink(lnk)
		if visit(c) {
			children, err := ds.GetLinks(ctx, c)
			if err != nil {
				if bestEffort && err == ErrNotFound {
					continue
				} else {
					return err
				}
			}
			err = EnumerateChildren(ctx, ds, children, visit, bestEffort)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func EnumerateChildrenAsync(ctx context.Context, ds DAGService, root *Node, visit func(*cid.Cid) bool) error {
	toprocess := make(chan []*cid.Cid, 8)
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

			var cids []*cid.Cid
			for _, lnk := range nd.Links {
				c := legacyCidFromLink(lnk)
				if visit(c) {
					live++
					cids = append(cids, c)
				}
			}

			if live == 0 {
				return nil
			}

			if len(cids) > 0 {
				select {
				case toprocess <- cids:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func fetchNodes(ctx context.Context, ds DAGService, in <-chan []*cid.Cid, out chan<- *NodeOption) {
	var wg sync.WaitGroup
	defer func() {
		// wait for all 'get' calls to complete so we don't accidentally send
		// on a closed channel
		wg.Wait()
		close(out)
	}()

	get := func(ks []*cid.Cid) {
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
