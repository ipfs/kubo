package ondemandpin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

var (
	ErrAlreadyRegistered = errors.New("CID is already registered for on-demand pinning")
	ErrNotRegistered     = errors.New("CID is not registered for on-demand pinning")
)

const dsPrefix = "/ondemand-pins/"

type Record struct {
	Cid cid.Cid `json:"Cid"`
	// PinnedByUs tracks the on-demand pins only (does not inlcude standard pins).
	PinnedByUs      bool      `json:"IsPinned"`
	LastAboveTarget time.Time `json:"LastAboveTarget,omitempty"`
	CreatedAt       time.Time `json:"CreatedAt"`
}

type Store struct {
	ds  datastore.Batching
	mu  sync.RWMutex
	now func() time.Time
}

func NewStore(ds datastore.Batching) *Store {
	return &Store{ds: ds, now: time.Now}
}

func dsKey(c cid.Cid) datastore.Key {
	return datastore.NewKey(dsPrefix + c.String())
}

func (s *Store) Add(ctx context.Context, c cid.Cid) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := dsKey(c)

	if has, err := s.ds.Has(ctx, key); err != nil {
		return fmt.Errorf("checking existing record: %w", err)
	} else if has {
		return fmt.Errorf("%s: %w", c, ErrAlreadyRegistered)
	}

	rec := Record{
		Cid:       c,
		CreatedAt: s.now(),
	}

	return s.put(ctx, key, &rec)
}

func (s *Store) Remove(ctx context.Context, c cid.Cid) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := dsKey(c)
	if has, err := s.ds.Has(ctx, key); err != nil {
		return fmt.Errorf("checking record: %w", err)
	} else if !has {
		return fmt.Errorf("%s: %w", c, ErrNotRegistered)
	}

	if err := s.ds.Delete(ctx, key); err != nil {
		return fmt.Errorf("deleting record: %w", err)
	}
	return s.ds.Sync(ctx, key)
}

func (s *Store) Get(ctx context.Context, c cid.Cid) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.get(ctx, dsKey(c))
}

func (s *Store) List(ctx context.Context) ([]Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results, err := s.ds.Query(ctx, query.Query{
		Prefix: dsPrefix,
	})
	if err != nil {
		return nil, fmt.Errorf("querying on-demand pins: %w", err)
	}
	defer results.Close()

	var records []Record
	for result := range results.Next() {
		if result.Error != nil {
			return nil, fmt.Errorf("iterating on-demand pins: %w", result.Error)
		}
		var rec Record
		if err := json.Unmarshal(result.Value, &rec); err != nil {
			return nil, fmt.Errorf("unmarshaling record %s: %w", result.Key, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

func (s *Store) Update(ctx context.Context, rec *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.put(ctx, dsKey(rec.Cid), rec)
}

func (s *Store) get(ctx context.Context, key datastore.Key) (*Record, error) {
	val, err := s.ds.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("getting record: %w", err)
	}
	var rec Record
	if err := json.Unmarshal(val, &rec); err != nil {
		return nil, fmt.Errorf("unmarshaling record: %w", err)
	}
	return &rec, nil
}

func (s *Store) put(ctx context.Context, key datastore.Key, rec *Record) error {
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}
	if err := s.ds.Put(ctx, key, val); err != nil {
		return fmt.Errorf("storing record: %w", err)
	}
	return s.ds.Sync(ctx, key)
}
