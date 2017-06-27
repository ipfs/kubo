// package merkledag implements the IPFS Merkle DAG datastructures.
package merkledag

import (
	"context"
	"fmt"
	"strings"
	"sync"

	blocks "github.com/ipfs/go-ipfs/blocks"
	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"

	ipldcbor "gx/ipfs/QmNrbCt8j9DT5W9Pmjy2SdudT9k8GpaDr4sRuFix3BXhgR/go-ipld-cbor"
	cid "gx/ipfs/QmYhQaCYEcaPPjxJX7YcPcVKkQfRy6sJ7B3XmGFk82XYdQ/go-cid"
	node "gx/ipfs/Qmb3Hm9QDFmfYuET4pu7Kyg8JV78jFa1nvZx5vnCZsK4ck/go-ipld-format"
)

var ErrNotFound = fmt.Errorf("merkledag: not found")

// DAGService is an IPFS Merkle DAG service.
type DAGService interface {
	Add(node.Node) (*cid.Cid, error)
	Get(context.Context, *cid.Cid) (node.Node, error)
	Remove(node.Node) error

	// GetDAG returns, in order, all the single leve child
	// nodes of the passed in node.
	GetMany(context.Context, []*cid.Cid) <-chan *NodeOption

	Batch() *Batch

	LinkService
}

type LinkService interface {
	// GetLinks return all links for a node.  The complete node does not
	// necessarily have to exist locally, or at all.  For example, raw
	// leaves cannot possibly have links so there is no need to look
	// at the node.
	GetLinks(context.Context, *cid.Cid) ([]*node.Link, error)

	GetOfflineLinkService() LinkService
}

func NewDAGService(bs bserv.BlockService) *dagService {
	return &dagService{Blocks: bs}
}

// dagService is an IPFS Merkle DAG service.
// - the root is virtual (like a forest)
// - stores nodes' data in a BlockService
// TODO: should cache Nodes that are in memory, and be
//       able to free some of them when vm pressure is high
type dagService struct {
	Blocks bserv.BlockService
}

// Add adds a node to the dagService, storing the block in the BlockService
func (n *dagService) Add(nd node.Node) (*cid.Cid, error) {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return nil, fmt.Errorf("dagService is nil")
	}

	return n.Blocks.AddBlock(nd)
}

func (n *dagService) Batch() *Batch {
	return &Batch{
		ds:      n,
		MaxSize: 8 << 20,

		// By default, only batch up to 128 nodes at a time.
		// The current implementation of flatfs opens this many file
		// descriptors at the same time for the optimized batch write.
		MaxBlocks: 128,
	}
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (n *dagService) Get(ctx context.Context, c *cid.Cid) (node.Node, error) {
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

	return decodeBlock(b)
}

func decodeBlock(b blocks.Block) (node.Node, error) {
	c := b.Cid()

	switch c.Type() {
	case cid.DagProtobuf:
		decnd, err := DecodeProtobuf(b.RawData())
		if err != nil {
			if strings.Contains(err.Error(), "Unmarshal failed") {
				return nil, fmt.Errorf("The block referred to by '%s' was not a valid merkledag node", c)
			}
			return nil, fmt.Errorf("Failed to decode Protocol Buffers: %v", err)
		}

		decnd.cached = b.Cid()
		decnd.Prefix = b.Cid().Prefix()
		return decnd, nil
	case cid.Raw:
		return NewRawNodeWPrefix(b.RawData(), b.Cid().Prefix())
	case cid.DagCBOR:
		return ipldcbor.Decode(b.RawData())
	default:
		return nil, fmt.Errorf("unrecognized object type: %s", c.Type())
	}
}

// GetLinks return the links for the node, the node doesn't necessarily have
// to exist locally.
func (n *dagService) GetLinks(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
	if c.Type() == cid.Raw {
		return nil, nil
	}
	node, err := n.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return node.Links(), nil
}

func (n *dagService) GetOfflineLinkService() LinkService {
	if n.Blocks.Exchange().IsOnline() {
		bsrv := bserv.New(n.Blocks.Blockstore(), offline.Exchange(n.Blocks.Blockstore()))
		return NewDAGService(bsrv)
	} else {
		return n
	}
}

func (n *dagService) Remove(nd node.Node) error {
	return n.Blocks.DeleteBlock(nd)
}

// GetLinksDirect creates a function to get the links for a node, from
// the node, bypassing the LinkService.  If the node does not exist
// locally (and can not be retrieved) an error will be returned.
func GetLinksDirect(serv node.NodeGetter) GetLinks {
	return func(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
		node, err := serv.Get(ctx, c)
		if err != nil {
			return nil, err
		}
		return node.Links(), nil
	}
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root *cid.Cid, serv DAGService) error {
	v, _ := ctx.Value("progress").(*ProgressTracker)
	if v == nil {
		return EnumerateChildrenAsync(ctx, GetLinksDirect(serv), root, cid.NewSet().Visit)
	}
	set := cid.NewSet()
	visit := func(c *cid.Cid) bool {
		if set.Visit(c) {
			v.Increment()
			return true
		} else {
			return false
		}
	}
	return EnumerateChildrenAsync(ctx, GetLinksDirect(serv), root, visit)
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
	Node node.Node
	Err  error
}

func (ds *dagService) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *NodeOption {
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

				nd, err := decodeBlock(b)
				if err != nil {
					out <- &NodeOption{Err: err}
					return
				}

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
func GetDAG(ctx context.Context, ds DAGService, root node.Node) []NodeGetter {
	var cids []*cid.Cid
	for _, lnk := range root.Links() {
		cids = append(cids, lnk.Cid)
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
		recv: make(chan node.Node, 1),
		ctx:  ctx,
		err:  make(chan error, 1),
	}
}

type nodePromise struct {
	cache node.Node
	clk   sync.Mutex
	recv  chan node.Node
	ctx   context.Context
	err   chan error
}

// NodeGetter provides a promise like interface for a dag Node
// the first call to Get will block until the Node is received
// from its internal channels, subsequent calls will return the
// cached node.
type NodeGetter interface {
	Get(context.Context) (node.Node, error)
	Fail(err error)
	Send(node.Node)
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

func (np *nodePromise) Send(nd node.Node) {
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

func (np *nodePromise) Get(ctx context.Context) (node.Node, error) {
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

	blocks    []blocks.Block
	size      int
	MaxSize   int
	MaxBlocks int
}

func (t *Batch) Add(nd node.Node) (*cid.Cid, error) {
	t.blocks = append(t.blocks, nd)
	t.size += len(nd.RawData())
	if t.size > t.MaxSize || len(t.blocks) > t.MaxBlocks {
		return nd.Cid(), t.Commit()
	}
	return nd.Cid(), nil
}

func (t *Batch) Commit() error {
	_, err := t.ds.Blocks.AddBlocks(t.blocks)
	t.blocks = nil
	t.size = 0
	return err
}

type GetLinks func(context.Context, *cid.Cid) ([]*node.Link, error)

// EnumerateChildren will walk the dag below the given root node and add all
// unseen children to the passed in set.
// TODO: parallelize to avoid disk latency perf hits?
func EnumerateChildren(ctx context.Context, getLinks GetLinks, root *cid.Cid, visit func(*cid.Cid) bool) error {
	links, err := getLinks(ctx, root)
	if err != nil {
		return err
	}
	for _, lnk := range links {
		c := lnk.Cid
		if visit(c) {
			err = EnumerateChildren(ctx, getLinks, c, visit)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type ProgressTracker struct {
	Total int
	lk    sync.Mutex
}

func (p *ProgressTracker) DeriveContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "progress", p)
}

func (p *ProgressTracker) Increment() {
	p.lk.Lock()
	defer p.lk.Unlock()
	p.Total++
}

func (p *ProgressTracker) Value() int {
	p.lk.Lock()
	defer p.lk.Unlock()
	return p.Total
}

// FetchGraphConcurrency is total number of concurrent fetches that
// 'fetchNodes' will start at a time
var FetchGraphConcurrency = 8

func EnumerateChildrenAsync(ctx context.Context, getLinks GetLinks, c *cid.Cid, visit func(*cid.Cid) bool) error {
	feed := make(chan *cid.Cid)
	out := make(chan []*node.Link)
	done := make(chan struct{})

	var setlk sync.Mutex

	errChan := make(chan error)
	fetchersCtx, cancel := context.WithCancel(ctx)

	defer cancel()

	for i := 0; i < FetchGraphConcurrency; i++ {
		go func() {
			for ic := range feed {
				links, err := getLinks(ctx, ic)
				if err != nil {
					errChan <- err
					return
				}

				setlk.Lock()
				unseen := visit(ic)
				setlk.Unlock()

				if unseen {
					select {
					case out <- links:
					case <-fetchersCtx.Done():
						return
					}
				}
				select {
				case done <- struct{}{}:
				case <-fetchersCtx.Done():
				}
			}
		}()
	}
	defer close(feed)

	send := feed
	var todobuffer []*cid.Cid
	var inProgress int

	next := c
	for {
		select {
		case send <- next:
			inProgress++
			if len(todobuffer) > 0 {
				next = todobuffer[0]
				todobuffer = todobuffer[1:]
			} else {
				next = nil
				send = nil
			}
		case <-done:
			inProgress--
			if inProgress == 0 && next == nil {
				return nil
			}
		case links := <-out:
			for _, lnk := range links {
				if next == nil {
					next = lnk.Cid
					send = feed
				} else {
					todobuffer = append(todobuffer, lnk.Cid)
				}
			}
		case err := <-errChan:
			return err

		case <-ctx.Done():
			return ctx.Err()
		}
	}

}
