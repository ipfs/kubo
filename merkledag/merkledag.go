// package merkledag implements the IPFS Merkle DAG datastructures.
package merkledag

import (
	"context"
	"fmt"
	"sync"

	bserv "github.com/ipfs/go-ipfs/blockservice"
	offline "github.com/ipfs/go-ipfs/exchange/offline"
	blocks "gx/ipfs/QmVA4mafxbfH5aEvNz8fyoxC6J1xhAtw88B4GerPznSZBg/go-block-format"

	ipldcbor "gx/ipfs/QmRsVKyuqssWQn4jtvH2PMA9Yc3AC9TYFFtvzCUbZG4kTT/go-ipld-cbor"
	cid "gx/ipfs/QmTprEaAA2A9bst5XH7exuyi5KzNMK3SEDNN8rBDnKWcUS/go-cid"
	node "gx/ipfs/QmVHxZ8ovAuHiHTbJa68budGYAqmMUzb1bqDW1SVb6y5M9/go-ipld-format"
)

// TODO: We should move these registrations elsewhere. Really, most of the IPLD
// functionality should go in a `go-ipld` repo but that will take a lot of work
// and design.
func init() {
	node.Register(cid.DagProtobuf, DecodeProtobufBlock)
	node.Register(cid.Raw, DecodeRawBlock)
	node.Register(cid.DagCBOR, ipldcbor.DecodeBlock)
}

var ErrNotFound = fmt.Errorf("merkledag: not found")

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

func (n *dagService) AddMany(nds []node.Node) ([]*cid.Cid, error) {
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
			return nil, ErrNotFound
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

func (n *dagService) OfflineNodeGetter() node.NodeGetter {
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
			if err == bserv.ErrNotFound {
				err = ErrNotFound
			}
			return nil, err
		}
		return node.Links(), nil
	}
}

type sesGetter struct {
	bs *bserv.Session
}

func (sg *sesGetter) Get(ctx context.Context, c *cid.Cid) (node.Node, error) {
	blk, err := sg.bs.GetBlock(ctx, c)
	if err != nil {
		return nil, err
	}

	return node.Decode(blk)
}

func (sg *sesGetter) OfflineNodeGetter() node.NodeGetter {
	bsrv := bserv.New(sg.bs.Blockstore(), offline.Exchange(sg.bs.Blockstore()))
	return NewDAGService(bsrv)
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

func (ds *dagService) GetMany(ctx context.Context, keys []*cid.Cid) <-chan *node.NodeOption {
	out := make(chan *node.NodeOption, len(keys))
	blocks := ds.Blocks.GetBlocks(ctx, keys)
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

type GetLinks func(context.Context, *cid.Cid) ([]*node.Link, error)

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
