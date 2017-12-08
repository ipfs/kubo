// Package namecache implements background following (resolution and pinning) of names
package namecache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	namesys "github.com/ipfs/go-ipfs/namesys"
	pin "github.com/ipfs/go-ipfs/pin"

	cid "gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	bstore "gx/ipfs/QmS2aqUZLJp8kF1ihE5rvDGE5LvmKDPnx32w9Z1BW9xLV5/go-ipfs-blockstore"
	uio "gx/ipfs/QmUnHNqhSB1JgzVCxL1Kz3yb4bdyB4q1Z9AD5AUBVmt3fZ/go-unixfs/io"
	path "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path"
	resolver "gx/ipfs/QmVi2uUygezqaMTqs3Yzt5FcZFHJoYD4B7jQ2BELjj7ZuY/go-path/resolver"
	ipld "gx/ipfs/QmcKKBwfz6FyQdHR2jsXrrF6XeSBXYL86anmWNewpFpoF5/go-ipld-format"
	logging "gx/ipfs/QmcuXC5cxs79ro2cUuHs4HQ2bkDLJUYokwL8aivcX6HW3C/go-log"
)

const (
	followInterval = 60 * time.Minute
	resolveTimeout = 60 * time.Second
)

var log = logging.Logger("namecache")

// NameCache represents a following cache of names
type NameCache interface {
	// Follow starts following name, pinning it if pinit is true
	Follow(name string, pinit bool) error
	// Unofollow cancels a follow
	Unfollow(name string) error
	// ListFollows returns a list of followed names
	ListFollows() []string
}

type nameCache struct {
	nsys    namesys.NameSystem
	pinning pin.Pinner
	dag     ipld.DAGService
	bstore  bstore.GCBlockstore

	ctx     context.Context
	follows map[string]func()
	mx      sync.Mutex
}

func NewNameCache(ctx context.Context, nsys namesys.NameSystem, pinning pin.Pinner, dag ipld.DAGService) NameCache {
	return &nameCache{
		ctx:     ctx,
		nsys:    nsys,
		pinning: pinning,
		dag:     dag,
		follows: make(map[string]func()),
	}
}

// Follow spawns a goroutine that periodically resolves a name
// and (when pinit is true) pins it in the background
func (nc *nameCache) Follow(name string, pinit bool) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	if _, ok := nc.follows[name]; ok {
		return fmt.Errorf("Already following %s", name)
	}

	ctx, cancel := context.WithCancel(nc.ctx)
	go nc.followName(ctx, name, pinit)
	nc.follows[name] = cancel

	return nil
}

// Unfollow cancels a follow
func (nc *nameCache) Unfollow(name string) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	cancel, ok := nc.follows[name]
	if ok {
		cancel()
		delete(nc.follows, name)
		return nil
	}

	return fmt.Errorf("Unknown name %s", name)
}

// ListFollows returns a list of names currently being followed
func (nc *nameCache) ListFollows() []string {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	follows := make([]string, 0)
	for name, _ := range nc.follows {
		follows = append(follows, name)
	}

	return follows
}

func (nc *nameCache) followName(ctx context.Context, name string, pinit bool) {
	// if cid != nil, we have created a new pin that is updated on changes and
	// unpinned on cancel
	c, err := nc.resolveAndPin(ctx, name, pinit)
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
				c, err = nc.resolveAndPin(ctx, name, pinit)
			}

			if err != nil {
				log.Errorf("Error following %s: %s", name, err.Error())
			}

		case <-ctx.Done():
			if c != cid.Undef {
				err = nc.unpin(c)
				if err != nil {
					log.Errorf("Error unpinning followed %s: %s", name, err.Error())
				}
			}
			return
		}
	}
}

func (nc *nameCache) resolveAndPin(ctx context.Context, name string, pinit bool) (cid.Cid, error) {
	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return cid.Undef, err
	}

	if !pinit {
		return cid.Undef, nil
	}

	c, err := pathToCid(ptr)
	if err != nil {
		return cid.Undef, err
	}

	defer nc.bstore.PinLock().Unlock()

	_, pinned, err := nc.pinning.IsPinned(c)
	if pinned || err != nil {
		return cid.Undef, err
	}

	n, err := nc.pathToNode(ctx, ptr)
	if err != nil {
		return cid.Undef, err
	}

	log.Debugf("pinning %s", c.String())

	err = nc.pinning.Pin(ctx, n, true)
	if err != nil {
		return cid.Undef, err
	}

	err = nc.pinning.Flush()

	return c, err
}

func (nc *nameCache) resolveAndUpdate(ctx context.Context, name string, c cid.Cid) (cid.Cid, error) {

	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return cid.Undef, err
	}

	ncid, err := pathToCid(ptr)
	if err != nil {
		return cid.Undef, err
	}

	if ncid.Equals(c) {
		return c, nil
	}

	defer nc.bstore.PinLock().Unlock()

	log.Debugf("Updating pin %s -> %s", c.String(), ncid.String())

	err = nc.pinning.Update(ctx, c, ncid, true)
	if err != nil {
		return c, err
	}

	err = nc.pinning.Flush()

	return ncid, err
}

func (nc *nameCache) unpin(cid cid.Cid) error {
	defer nc.bstore.PinLock().Unlock()

	err := nc.pinning.Unpin(nc.ctx, cid, true)
	if err != nil {
		return err
	}

	return nc.pinning.Flush()
}

func (nc *nameCache) resolve(ctx context.Context, name string) (path.Path, error) {
	log.Debugf("resolving %s", name)

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

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

func (nc *nameCache) pathToNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	r := &resolver.Resolver{
		DAG:         nc.dag,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	return r.ResolvePath(ctx, p)
}
