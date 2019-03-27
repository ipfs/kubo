package provider

import (
	"context"
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/namespace"
	"github.com/ipfs/go-datastore/query"
)

const providerTrackingPrefix = "/provider/tracking/"

// Tracker keeps track of which cids are being provided.
type Tracker struct {
	datastore ds.Datastore
}

// NewTracker creates a simple tracker from any datastore
func NewTracker(datastore ds.Datastore) *Tracker {
	return &Tracker{
		datastore: namespace.Wrap(datastore, ds.NewKey(providerTrackingPrefix)),
	}
}

// IsTracking returns whether or not the given cid is being tracked
func (t *Tracker) IsTracking(cid cid.Cid) (bool, error) {
	return t.datastore.Has(providerTrackingKey(cid))
}

// Track registers the given cid with the tracker
func (t *Tracker) Track(cid cid.Cid) error {
	return t.datastore.Put(providerTrackingKey(cid), cid.Bytes())
}

// Untrack unregisters the given cid with the tracker
func (t *Tracker) Untrack(cid cid.Cid) error {
	return t.datastore.Delete(providerTrackingKey(cid))
}

// Tracking returns all the cids that are registered with the tracker
func (t *Tracker) Tracking(ctx context.Context) (<-chan cid.Cid, error) {
	q := query.Query{}
	results, err := t.datastore.Query(q)
	if err != nil {
		return nil, err
	}
	cids := make(chan cid.Cid)
	go func() {
		defer close(cids)
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
