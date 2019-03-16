// Package namecache implements background following (resolution and pinning) of names
package namecache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/dagutils"
	"github.com/ipfs/go-ipfs/namesys"

	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	"gx/ipfs/QmWqh9oob7ZHQRwU5CdTqpnC8ip8BEkFNrwXRxeNo5Y7vA/go-path"
	dag "gx/ipfs/Qmb2UEG2TAeVrEJSjqsZF7Y2he7wRDkrdt6c3bECxwZf4k/go-merkledag"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
)

const (
	DefaultFollowInterval = 1 * time.Hour
	resolveTimeout        = 1 * time.Minute
)

var log = logging.Logger("namecache")

// NameCache represents a following cache of names
type NameCache interface {
	// Follow starts following name
	Follow(name string, prefetch bool, followInterval time.Duration) error
	// Unofollow cancels a follow
	Unfollow(name string) error
	// ListFollows returns a list of followed names
	ListFollows() []string
}

type nameCache struct {
	nsys    namesys.NameSystem
	dag     ipld.NodeGetter

	ctx     context.Context
	follows map[string]func()
	mx      sync.Mutex
}

func NewNameCache(ctx context.Context, nsys namesys.NameSystem, dag ipld.NodeGetter) NameCache {
	return &nameCache{
		ctx:     ctx,
		nsys:    nsys,
		dag:     dag,
		follows: make(map[string]func()),
	}
}

// Follow spawns a goroutine that periodically resolves a name
// and (when dopin is true) pins it in the background
func (nc *nameCache) Follow(name string, prefetch bool, followInterval time.Duration) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	if _, ok := nc.follows[name]; ok {
		return fmt.Errorf("already following %s", name)
	}

	ctx, cancel := context.WithCancel(nc.ctx)
	go nc.followName(ctx, name, prefetch, followInterval)
	nc.follows[name] = cancel

	return nil
}

// Unfollow cancels a follow
func (nc *nameCache) Unfollow(name string) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	cancel, ok := nc.follows[name]
	if !ok {
		return fmt.Errorf("unknown name %s", name)
	}

	cancel()
	delete(nc.follows, name)
	return nil
}

// ListFollows returns a list of names currently being followed
func (nc *nameCache) ListFollows() []string {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	follows := make([]string, 0, len(nc.follows))
	for name := range nc.follows {
		follows = append(follows, name)
	}

	return follows
}

func (nc *nameCache) followName(ctx context.Context, name string, prefetch bool, followInterval time.Duration) {
	emptynode := new(dag.ProtoNode)

	c, err := nc.resolveAndUpdate(ctx, name, prefetch, emptynode.Cid())
	if err != nil {
		log.Errorf("Error following %s: %s", name, err.Error())
	}

	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c, err = nc.resolveAndUpdate(ctx, name, prefetch, c)

			if err != nil {
				log.Errorf("Error following %s: %s", name, err.Error())
			}

		case <-ctx.Done():
			return
		}
	}
}

func (nc *nameCache) resolveAndUpdate(ctx context.Context, name string, prefetch bool, oldcid cid.Cid) (cid.Cid, error) {
	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return cid.Undef, err
	}

	newcid, err := pathToCid(ptr)
	if err != nil {
		return cid.Undef, err
	}

	if newcid.Equals(oldcid) || !prefetch {
		return newcid, nil
	}

	oldnd, err := nc.dag.Get(ctx, oldcid)
	if err != nil {
		return cid.Undef, err
	}

	newnd, err := nc.dag.Get(ctx, newcid)
	if err != nil {
		return cid.Undef, err
	}

	changes, err := dagutils.Diff(ctx, nc.dag, oldnd, newnd)
	if err != nil {
		return cid.Undef, err
	}

	log.Debugf("fetching changes in %s (%s -> %s)", name, oldcid, newcid)
	for _, change := range changes {
		if change.Type == iface.DiffRemove {
			continue
		}

		toFetch, err := nc.dag.Get(ctx, change.After)
		if err != nil {
			return cid.Undef, err
		}

		// just iterate over all nodes
		walker := ipld.NewWalker(ctx, ipld.NewNavigableIPLDNode(toFetch, nc.dag))
		if err := walker.Iterate(func(node ipld.NavigableNode) error {
			return nil
		}); err != ipld.EndOfDag {
			return cid.Undef, fmt.Errorf("unexpected error when prefetching followed name: %s", err)
		}
	}

	return newcid, err
}

func (nc *nameCache) resolve(ctx context.Context, name string) (path.Path, error) {
	log.Debugf("resolving %s", name)

	rctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	p, err := nc.nsys.Resolve(rctx, name)
	if err != nil {
		return "", err
	}

	log.Debugf("resolved %s to %s", name, p)

	return p, nil
}

func pathToCid(p path.Path) (cid.Cid, error) {
	return cid.Decode(p.Segments()[1])
}
