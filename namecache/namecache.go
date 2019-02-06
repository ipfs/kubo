// Package namecache implements background following (resolution and pinning) of names
package namecache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	namesys "github.com/ipfs/go-ipfs/namesys"

	uio "gx/ipfs/QmQ1JnYpnzkaurjW1yxkQxC2w3K1PorNE1nv1vaP5Le7sq/go-unixfs/io"
	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	path "gx/ipfs/QmWqh9oob7ZHQRwU5CdTqpnC8ip8BEkFNrwXRxeNo5Y7vA/go-path"
	resolver "gx/ipfs/QmWqh9oob7ZHQRwU5CdTqpnC8ip8BEkFNrwXRxeNo5Y7vA/go-path/resolver"
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
	bstore  bstore.GCBlockstore

	ctx     context.Context
	follows map[string]func()
	mx      sync.Mutex
}

func NewNameCache(ctx context.Context, nsys namesys.NameSystem, dag ipld.NodeGetter, bstore bstore.GCBlockstore) NameCache {
	return &nameCache{
		ctx:     ctx,
		nsys:    nsys,
		dag:     dag,
		bstore:  bstore,
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
		return fmt.Errorf("Already following %s", name)
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
		return fmt.Errorf("Unknown name %s", name)
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
	// if cid != nil, we have prefetched data under the node
	c, err := nc.resolveAndFetch(ctx, name, prefetch)
	if err != nil {
		log.Errorf("Error following %s: %s", name, err.Error())
	}

	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c != cid.Undef {
				c, err = nc.resolveAndUpdate(ctx, name, c)
			} else {
				c, err = nc.resolveAndFetch(ctx, name, prefetch)
			}

			if err != nil {
				log.Errorf("Error following %s: %s", name, err.Error())
			}

		case <-ctx.Done():
			return
		}
	}
}

func (nc *nameCache) resolveAndFetch(ctx context.Context, name string, prefetch bool) (cid.Cid, error) {
	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return cid.Undef, err
	}

	if !prefetch {
		return cid.Undef, nil
	}

	c, err := pathToCid(ptr)
	if err != nil {
		return cid.Undef, err
	}

	defer nc.bstore.PinLock().Unlock()

	n, err := nc.pathToNode(ctx, ptr)
	if err != nil {
		return cid.Undef, err
	}

	return c, err
}

func (nc *nameCache) resolveAndUpdate(ctx context.Context, name string, oldcid cid.Cid) (cid.Cid, error) {
	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return cid.Undef, err
	}

	newcid, err := pathToCid(ptr)
	if err != nil {
		return cid.Undef, err
	}

	if newcid.Equals(oldcid) {
		return oldcid, nil
	}

	// TODO: handle prefetching

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

	// TODO: handle prefetching

	return p, nil
}

func pathToCid(p path.Path) (cid.Cid, error) {
	return cid.Decode(p.Segments()[1])
}

func (nc *nameCache) pathToNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	r := &resolver.Resolver{
		DAG:         nc.dag,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	return r.ResolvePath(ctx, p)
}
