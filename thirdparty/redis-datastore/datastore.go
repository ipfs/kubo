package redis

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/fzzy/radix/redis"
	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	query "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var _ datastore.Datastore = &Datastore{}
var _ datastore.ThreadSafeDatastore = &Datastore{}

var ErrInvalidType = errors.New("redis datastore: invalid type error. this datastore only supports []byte values")

func NewExpiringDatastore(client *redis.Client, ttl time.Duration) (datastore.ThreadSafeDatastore, error) {
	return &Datastore{
		client: client,
		ttl:    ttl,
	}, nil
}

func NewDatastore(client *redis.Client) (datastore.ThreadSafeDatastore, error) {
	return &Datastore{
		client: client,
	}, nil
}

type Datastore struct {
	mu     sync.Mutex
	client *redis.Client
	ttl    time.Duration
}

func (ds *Datastore) Put(key datastore.Key, value interface{}) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	data, ok := value.([]byte)
	if !ok {
		return ErrInvalidType
	}

	ds.client.Append("SET", key.String(), data)
	if ds.ttl != 0 {
		ds.client.Append("EXPIRE", key.String(), ds.ttl.Seconds())
	}
	if err := ds.client.GetReply().Err; err != nil {
		return fmt.Errorf("failed to put value: %s", err)
	}
	if ds.ttl != 0 {
		if err := ds.client.GetReply().Err; err != nil {
			return fmt.Errorf("failed to set expiration: %s", err)
		}
	}
	return nil
}

func (ds *Datastore) Get(key datastore.Key) (value interface{}, err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("GET", key.String()).Bytes()
}

func (ds *Datastore) Has(key datastore.Key) (exists bool, err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("EXISTS", key.String()).Bool()
}

func (ds *Datastore) Delete(key datastore.Key) (err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("DEL", key.String()).Err
}

func (ds *Datastore) Query(q query.Query) (query.Results, error) {
	return nil, errors.New("TODO implement query for redis datastore?")
}

func (ds *Datastore) IsThreadSafe() {}
