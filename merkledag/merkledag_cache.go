// package merkledag implements the ipfs Merkle DAG datastructures.
package merkledag

import (
	key "github.com/ipfs/go-ipfs/blocks/key"
	"gx/ipfs/QmVYxfoJQiZijTgPNHCHgHELvQpbsJNTg6Crmc3dQkj3yy/golang-lru"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
)

func NewDAGCache(ds DAGService, size int) (DAGService, error) {
	cache, err := lru.New(size)
	if err != err {
		return nil, err
	}
	return &dagCache{ds, cache}, nil
}

type dagCache struct {
	DAGService
	cache *lru.Cache
}

func CloneNode(nd * Node) *Node {
	// this makes a shallow copy
	//return &Node{nd.Links, nd.Data, nd.encoded, nd.cached}
	// this makes a deeper copy
	return nd.Copy()
}

func (ds *dagCache) Add(node *Node) (key.Key, error) {
	key, err := ds.DAGService.Add(node)
	if err == nil && len(node.Links) > 0 {
		ds.cache.Add(key, CloneNode(node))
		//println("cache add", key.String())
		return key, err
	}
	return key, err
}

// Get retrieves a node from the dagService, fetching the block in the BlockService
func (ds *dagCache) Get(ctx context.Context, k key.Key) (*Node, error) {
	//return ds.DAGService.Get(ctx, k)
	if node, ok := ds.cache.Get(k); ok {
		//println("cache hit", k.String())
		return CloneNode(node.(*Node)), nil
	}
	node, err := ds.DAGService.Get(ctx, k)
	if err == nil && len(node.Links) > 0 {	
		//println("cache miss", k.String())
		ds.cache.Add(k, CloneNode(node))
	}
	return node, err
}

func (ds *dagCache) GetMany(ctx context.Context, keys []key.Key) <-chan *NodeOption {
	out := make(chan *NodeOption, len(keys))

	stillNeed := make([]key.Key, 0, len(keys))

	for _, key := range keys {
		if node, ok := ds.cache.Get(key); ok {
			//println("cache hit (many)", key.String())
			out <- &NodeOption{Node: CloneNode(node.(*Node))}
		} else {
			//println("cache miss (many) ", key.String())
			stillNeed = append(stillNeed, key)
		}
	}

	dags := ds.DAGService.GetMany(ctx, stillNeed)

	go func() {
		defer close(out)
		for n := range dags {
			if n.Err == nil && n.Node != nil {
				key, err := n.Node.Key()
				if err == nil && len(n.Node.Links) > 0 {
					ds.cache.Add(key, CloneNode(n.Node))
				}
			}
			out <- n
		}
	}()
	return out
}
