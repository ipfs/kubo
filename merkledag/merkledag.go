// package merkledag implements the IPFS Merkle DAG datastructures.
package merkledag

import (
	"context"
	"fmt"
	"sync"

	bserv "github.com/ipfs/go-ipfs/blockservice"

	ipldcbor "gx/ipfs/QmNRz7BDWfdFNVLt7AVvmRefkrURD25EeoipcXqo6yoXU1/go-ipld-cbor"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	node "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
	blocks "gx/ipfs/Qmej7nf81hi2x2tvjRBF3mcp74sQyuDH4VMYDGd1YtXjb2/go-block-format"
)

// TODO: We should move these registrations elsewhere. Really, most of the IPLD
// functionality should go in a `go-ipld` repo but that will take a lot of work
// and design.
func init() {
	node.Register(cid.DagProtobuf, DecodeProtobufBlock)
	node.Register(cid.Raw, DecodeRawBlock)
	node.Register(cid.DagCBOR, ipldcbor.DecodeBlock)
}

// NewDAGService constructs a new DAGService (using the default implementation).
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
func (n *dagService) Add(ctx context.Context, nd node.Node) error {
	if n == nil { // FIXME remove this assertion. protect with constructor invariant
		return fmt.Errorf("dagService is nil")
	}

	return n.Blocks.AddBlock(nd)
}

func (n *dagService) AddMany(ctx context.Context, nds []node.Node) error {
	blks := make([]blocks.Block, len(nds))
	for i, nd := range nds {
		blks[i] = nd
	}
	return n.Blocks.AddBlocks(blks)
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
			return nil, node.ErrNotFound
		}
		return nil, fmt.Errorf("Failed to get block for %s: %v", c, err)
	}

	return node.Decode(b)
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

func (n *dagService) Remove(ctx context.Context, c *cid.Cid) error {
	return n.Blocks.DeleteBlock(c)
}

// RemoveMany removes multiple nodes from the DAG. It will likely be faster than
// removing them individually.
//
// This operation is not atomic. If it returns an error, some nodes may or may
// not have been removed.
func (n *dagService) RemoveMany(ctx context.Context, cids []*cid.Cid) error {
	// TODO(#4608): make this batch all the way down.
	for _, c := range cids {
		if err := n.Blocks.DeleteBlock(c); err != nil {
			return err
		}
	}
	return nil
}

// GetLinksDirect creates a function to get the links for a node, from
// the node, bypassing the LinkService.  If the node does not exist
// locally (and can not be retrieved) an error will be returned.
func GetLinksDirect(serv node.NodeGetter) GetLinks {
	return func(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
		nd, err := serv.Get(ctx, c)
		if err != nil {
			if err == bserv.ErrNotFound {
				err = node.ErrNotFound
			}
			return nil, err
		}
		return nd.Links(), nil
	}
}

type sesGetter struct {
	bs *bserv.Session
}

// Get gets a single node from the DAG.
func (sg *sesGetter) Get(ctx context.Context, c *cid.Cid) (node.Node, error) {
	blk, err := sg.bs.GetBlock(ctx, c)
	switch err {
	case bserv.ErrNotFound:
		return nil, node.ErrNotFound
	default:
		return nil, err
	case nil:
		// noop
	}

	return node.Decode(blk)
}

// GetMany gets many nodes at once, batching the request if possible.
func (sg *sesGetter) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *node.NodeOption {
	return getNodesFromBG(ctx, sg.bs, keys)
}

// FetchGraph fetches all nodes that are children of the given node
func FetchGraph(ctx context.Context, root *cid.Cid, serv node.DAGService) error {
	var ng node.NodeGetter = serv
	ds, ok := serv.(*dagService)
	if ok {
		ng = &sesGetter{bserv.NewSession(ctx, ds.Blocks)}
	}

	v, _ := ctx.Value("progress").(*ProgressTracker)
	if v == nil {
		return EnumerateChildrenAsync(ctx, GetLinksDirect(ng), root, cid.NewSet().Visit)
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
	return EnumerateChildrenAsync(ctx, GetLinksDirect(ng), root, visit)
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

// GetMany gets many nodes from the DAG at once.
//
// This method may not return all requested nodes (and may or may not return an
// error indicating that it failed to do so. It is up to the caller to verify
// that it received all nodes.
func (n *dagService) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *node.NodeOption {
	return getNodesFromBG(ctx, n.Blocks, keys)
}

func getNodesFromBG(ctx context.Context, bs bserv.BlockGetter, keys []*cid.Cid) <-chan *node.NodeOption {
	out := make(chan *node.NodeOption, len(keys))
	blocks := bs.GetBlocks(ctx, keys)
	var count int

	go func() {
		defer close(out)
		for {
			select {
			case b, ok := <-blocks:
				if !ok {
					if count != len(keys) {
						out <- &node.NodeOption{Err: fmt.Errorf("failed to fetch all nodes")}
					}
					return
				}

				nd, err := node.Decode(b)
				if err != nil {
					out <- &node.NodeOption{Err: err}
					return
				}

				out <- &node.NodeOption{Node: nd}
				count++

			case <-ctx.Done():
				out <- &node.NodeOption{Err: ctx.Err()}
				return
			}
		}
	}()
	return out
}

// GetLinks is the type of function passed to the EnumerateChildren function(s)
// for getting the children of an IPLD node.
type GetLinks func(context.Context, *cid.Cid) ([]*node.Link, error)

// GetLinksWithDAG returns a GetLinks function that tries to use the given
// NodeGetter as a LinkGetter to get the children of a given IPLD node. This may
// allow us to traverse the DAG without actually loading and parsing the node in
// question (if we already have the links cached).
func GetLinksWithDAG(ng node.NodeGetter) GetLinks {
	return func(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
		return node.GetLinks(ctx, ng, c)
	}
}

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

// EnumerateChildrenAsync is equivalent to EnumerateChildren *except* that it
// fetches children in parallel.
//
// NOTE: It *does not* make multiple concurrent calls to the passed `visit` function.
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

var _ node.LinkGetter = &dagService{}
var _ node.NodeGetter = &dagService{}
var _ node.NodeGetter = &sesGetter{}
var _ node.DAGService = &dagService{}
