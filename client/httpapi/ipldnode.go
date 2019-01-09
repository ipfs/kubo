package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/ipfs/go-ipfs/core/coreapi/interface"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	ipfspath "github.com/ipfs/go-path"
)

type ipldNode struct {
	ctx  context.Context //TODO: should we re-consider adding ctx to ipld interfaces?
	path iface.ResolvedPath
	api  *HttpApi
}

func (a *HttpApi) nodeFromPath(ctx context.Context, p iface.ResolvedPath) ipld.Node {
	return &ipldNode{
		ctx:  ctx,
		path: p,
		api:  a,
	}
}

func (n *ipldNode) RawData() []byte {
	r, err := n.api.Block().Get(n.ctx, n.path)
	if err != nil {
		panic(err) // TODO: eww, should we add errors too / better ideas?
	}

	b, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}

	return b
}

func (n *ipldNode) Cid() cid.Cid {
	return n.path.Cid()
}

func (n *ipldNode) String() string {
	return fmt.Sprintf("[Block %s]", n.Cid())
}

func (n *ipldNode) Loggable() map[string]interface{} {
	return nil //TODO: we can't really do better here, can we?
}

// TODO: should we use 'full'/real ipld codecs for this? js-ipfs-api does that.
// We can also give people a choice
func (n *ipldNode) Resolve(path []string) (interface{}, []string, error) {
	p := ipfspath.Join([]string{n.path.String(), ipfspath.Join(path)})

	var out interface{}
	n.api.request("dag/get", p).Exec(n.ctx, &out)

	// TODO: this is more than likely wrong, fix if we decide to stick with this 'http-ipld-node' hack
	for len(path) > 0 {
		switch o := out.(type) {
		case map[string]interface{}:
			v, ok := o[path[0]]
			if !ok {
				// TODO: ipld links
				return nil, nil, errors.New("no element under this path")
			}
			out = v
		case []interface{}:
			n, err := strconv.ParseUint(path[0], 10, 32)
			if err != nil {
				return nil, nil, err
			}
			if len(o) < int(n) {
				return nil, nil, errors.New("no element under this path")
			}
			out = o[n]
		}
		path = path[1:]
	}

	return out, path, nil
}

func (n *ipldNode) Tree(path string, depth int) []string {
	panic("implement me")
}

func (n *ipldNode) ResolveLink(path []string) (*ipld.Link, []string, error) {
	panic("implement me")
}

func (n *ipldNode) Copy() ipld.Node {
	panic("implement me")
}

func (n *ipldNode) Links() []*ipld.Link {
	panic("implement me")
}

func (n *ipldNode) Stat() (*ipld.NodeStat, error) {
	panic("implement me")
}

func (n *ipldNode) Size() (uint64, error) {
	panic("implement me")
}

var _ ipld.Node = &ipldNode{}
