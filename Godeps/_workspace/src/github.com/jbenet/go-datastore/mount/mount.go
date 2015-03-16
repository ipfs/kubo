// Package mount provides a Datastore that has other Datastores
// mounted at various key prefixes.
package mount

import (
	"errors"
	"strings"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var (
	ErrNoMount = errors.New("no datastore mounted for this key")
)

type Mount struct {
	Prefix    datastore.Key
	Datastore datastore.Datastore
}

func New(mounts []Mount) *Datastore {
	// make a copy so we're sure it doesn't mutate
	m := make([]Mount, len(mounts))
	for i, v := range mounts {
		m[i] = v
	}
	return &Datastore{mounts: m}
}

type Datastore struct {
	mounts []Mount
}

var _ datastore.Datastore = (*Datastore)(nil)

func (d *Datastore) lookup(key datastore.Key) (ds datastore.Datastore, mountpoint, rest datastore.Key) {
	for _, m := range d.mounts {
		if m.Prefix.Equal(key) || m.Prefix.IsAncestorOf(key) {
			s := strings.TrimPrefix(key.String(), m.Prefix.String())
			k := datastore.NewKey(s)
			return m.Datastore, m.Prefix, k
		}
	}
	return nil, datastore.NewKey("/"), key
}

func (d *Datastore) Put(key datastore.Key, value interface{}) error {
	ds, _, k := d.lookup(key)
	if ds == nil {
		return ErrNoMount
	}
	return ds.Put(k, value)
}

func (d *Datastore) Get(key datastore.Key) (value interface{}, err error) {
	ds, _, k := d.lookup(key)
	if ds == nil {
		return nil, datastore.ErrNotFound
	}
	return ds.Get(k)
}

func (d *Datastore) Has(key datastore.Key) (exists bool, err error) {
	ds, _, k := d.lookup(key)
	if ds == nil {
		return false, nil
	}
	return ds.Has(k)
}

func (d *Datastore) Delete(key datastore.Key) error {
	ds, _, k := d.lookup(key)
	if ds == nil {
		return datastore.ErrNotFound
	}
	return ds.Delete(k)
}

func (d *Datastore) Query(q query.Query) (query.Results, error) {
	if len(q.Filters) > 0 ||
		len(q.Orders) > 0 ||
		q.Limit > 0 ||
		q.Offset > 0 {
		// TODO this is overly simplistic, but the only caller is
		// `ipfs refs local` for now, and this gets us moving.
		return nil, errors.New("mount only supports listing all prefixed keys in random order")
	}
	key := datastore.NewKey(q.Prefix)
	ds, mount, k := d.lookup(key)
	if ds == nil {
		return nil, errors.New("mount only supports listing a mount point")
	}
	// TODO support listing cross mount points too

	// delegate the query to the mounted datastore, while adjusting
	// keys in and out
	q2 := q
	q2.Prefix = k.String()
	wrapDS := keytransform.Wrap(ds, &keytransform.Pair{
		Convert: func(datastore.Key) datastore.Key {
			panic("this should never be called")
		},
		Invert: func(k datastore.Key) datastore.Key {
			return mount.Child(k)
		},
	})

	r, err := wrapDS.Query(q2)
	if err != nil {
		return nil, err
	}
	r = query.ResultsReplaceQuery(r, q)
	return r, nil
}
