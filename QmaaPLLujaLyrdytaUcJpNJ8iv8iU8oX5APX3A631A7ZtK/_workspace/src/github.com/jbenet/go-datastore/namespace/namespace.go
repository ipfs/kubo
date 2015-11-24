package namespace

import (
	"fmt"
	"strings"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ktds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
	dsq "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

// PrefixTransform constructs a KeyTransform with a pair of functions that
// add or remove the given prefix key.
//
// Warning: will panic if prefix not found when it should be there. This is
// to avoid insidious data inconsistency errors.
func PrefixTransform(prefix ds.Key) ktds.KeyTransform {
	return &ktds.Pair{

		// Convert adds the prefix
		Convert: func(k ds.Key) ds.Key {
			return prefix.Child(k)
		},

		// Invert removes the prefix. panics if prefix not found.
		Invert: func(k ds.Key) ds.Key {
			if !prefix.IsAncestorOf(k) {
				fmt.Errorf("Expected prefix (%s) in key (%s)", prefix, k)
				panic("expected prefix not found")
			}

			s := strings.TrimPrefix(k.String(), prefix.String())
			return ds.NewKey(s)
		},
	}
}

// Wrap wraps a given datastore with a key-prefix.
func Wrap(child ds.Datastore, prefix ds.Key) *datastore {
	if child == nil {
		panic("child (ds.Datastore) is nil")
	}

	d := ktds.Wrap(child, PrefixTransform(prefix))
	return &datastore{Datastore: d, raw: child, prefix: prefix}
}

type datastore struct {
	prefix ds.Key
	raw    ds.Datastore
	ktds.Datastore
}

// Query implements Query, inverting keys on the way back out.
func (d *datastore) Query(q dsq.Query) (dsq.Results, error) {
	qr, err := d.raw.Query(q)
	if err != nil {
		return nil, err
	}

	ch := make(chan dsq.Result)
	go func() {
		defer close(ch)
		defer qr.Close()

		for r := range qr.Next() {
			if r.Error != nil {
				ch <- r
				continue
			}

			k := ds.NewKey(r.Entry.Key)
			if !d.prefix.IsAncestorOf(k) {
				continue
			}

			r.Entry.Key = d.Datastore.InvertKey(k).String()
			ch <- r
		}
	}()

	return dsq.DerivedResults(qr, ch), nil
}

func (d *datastore) Batch() (ds.Batch, error) {
	if bds, ok := d.Datastore.(ds.Batching); ok {
		return bds.Batch()
	}

	return nil, ds.ErrBatchUnsupported
}
