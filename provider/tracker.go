package provider

import (
	"context"
	"gx/ipfs/QmR8BauakNcBa3RbE4nbQu76PDiJgoQgz8AJdhJuiU4TAw/go-cid"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/namespace"
	"gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore/query"
	ds "gx/ipfs/Qmf4xQhNomPNhrtZc67qSnfJSjxjXs9LWvknJtSXwimPrM/go-datastore"
)

const providerTrackingPrefix = "/provider/tracking/"

// Keeps track of which cids are being provided.
type Tracker struct {
	datastore ds.Datastore
}

func NewTracker(datastore ds.Datastore) *Tracker {
	return &Tracker{
		datastore: namespace.Wrap(datastore, ds.NewKey(providerTrackingPrefix)),
	}
}

func (t *Tracker) IsTracking(cid cid.Cid) (bool, error) {
	return t.datastore.Has(providerTrackingKey(cid))
}

func (t *Tracker) Track(cid cid.Cid) error {
	return t.datastore.Put(providerTrackingKey(cid), cid.Bytes())
}

func (t *Tracker) Untrack(cid cid.Cid) error {
	return t.datastore.Delete(providerTrackingKey(cid))
}

// Returns all the cids that are being tracked.
func (t *Tracker) Tracking(ctx context.Context) (<-chan cid.Cid, error) {
	q := query.Query{}
	results, err := t.datastore.Query(q)
	if err != nil {
		return nil, err
	}
	cids := make(chan cid.Cid)
	go func() {
		defer close(cids);
		for result := range results.Next() {
			key, err := cid.Parse(result.Value)
			if err != nil {
				log.Warningf("unable to parse tracked cid: %s", err)
			}
			select {
			case <-ctx.Done():
				return
			case cids <- key:
			}
		}
	}()
	return cids, nil
}

func providerTrackingKey(cid cid.Cid) ds.Key {
	return ds.NewKey(cid.String())
}
