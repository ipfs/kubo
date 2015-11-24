// Package mount provides a Datastore that has other Datastores
// mounted at various key prefixes and is threadsafe
package syncmount

import (
	"errors"
	"io"
	"strings"
	"sync"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var (
	ErrNoMount = errors.New("no datastore mounted for this key")
)

type Mount struct {
	Prefix    ds.Key
	Datastore ds.Datastore
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
	lk     sync.Mutex
}

var _ ds.Datastore = (*Datastore)(nil)

func (d *Datastore) lookup(key ds.Key) (ds.Datastore, ds.Key, ds.Key) {
	d.lk.Lock()
	defer d.lk.Unlock()
	for _, m := range d.mounts {
		if m.Prefix.Equal(key) || m.Prefix.IsAncestorOf(key) {
			s := strings.TrimPrefix(key.String(), m.Prefix.String())
			k := ds.NewKey(s)
			return m.Datastore, m.Prefix, k
		}
	}
	return nil, ds.NewKey("/"), key
}

func (d *Datastore) Put(key ds.Key, value interface{}) error {
	cds, _, k := d.lookup(key)
	if cds == nil {
		return ErrNoMount
	}
	return cds.Put(k, value)
}

func (d *Datastore) Get(key ds.Key) (value interface{}, err error) {
	cds, _, k := d.lookup(key)
	if cds == nil {
		return nil, ds.ErrNotFound
	}
	return cds.Get(k)
}

func (d *Datastore) Has(key ds.Key) (exists bool, err error) {
	cds, _, k := d.lookup(key)
	if cds == nil {
		return false, nil
	}
	return cds.Has(k)
}

func (d *Datastore) Delete(key ds.Key) error {
	cds, _, k := d.lookup(key)
	if cds == nil {
		return ds.ErrNotFound
	}
	return cds.Delete(k)
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
	key := ds.NewKey(q.Prefix)
	cds, mount, k := d.lookup(key)
	if cds == nil {
		return nil, errors.New("mount only supports listing a mount point")
	}
	// TODO support listing cross mount points too

	// delegate the query to the mounted datastore, while adjusting
	// keys in and out
	q2 := q
	q2.Prefix = k.String()
	wrapDS := keytransform.Wrap(cds, &keytransform.Pair{
		Convert: func(ds.Key) ds.Key {
			panic("this should never be called")
		},
		Invert: func(k ds.Key) ds.Key {
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

func (d *Datastore) IsThreadSafe() {}

func (d *Datastore) Close() error {
	for _, d := range d.mounts {
		if c, ok := d.Datastore.(io.Closer); ok {
			err := c.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type mountBatch struct {
	mounts map[string]ds.Batch
	lk     sync.Mutex

	d *Datastore
}

func (d *Datastore) Batch() (ds.Batch, error) {
	return &mountBatch{
		mounts: make(map[string]ds.Batch),
		d:      d,
	}, nil
}

func (mt *mountBatch) lookupBatch(key ds.Key) (ds.Batch, ds.Key, error) {
	mt.lk.Lock()
	defer mt.lk.Unlock()

	child, loc, rest := mt.d.lookup(key)
	t, ok := mt.mounts[loc.String()]
	if !ok {
		bds, ok := child.(ds.Batching)
		if !ok {
			return nil, ds.NewKey(""), ds.ErrBatchUnsupported
		}
		var err error
		t, err = bds.Batch()
		if err != nil {
			return nil, ds.NewKey(""), err
		}
		mt.mounts[loc.String()] = t
	}
	return t, rest, nil
}

func (mt *mountBatch) Put(key ds.Key, val interface{}) error {
	t, rest, err := mt.lookupBatch(key)
	if err != nil {
		return err
	}

	return t.Put(rest, val)
}

func (mt *mountBatch) Delete(key ds.Key) error {
	t, rest, err := mt.lookupBatch(key)
	if err != nil {
		return err
	}

	return t.Delete(rest)
}

func (mt *mountBatch) Commit() error {
	for _, t := range mt.mounts {
		err := t.Commit()
		if err != nil {
			return err
		}
	}
	return nil
}
