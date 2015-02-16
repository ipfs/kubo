package redis

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/fzzy/radix/redis"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	query "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var _ datastore.Datastore = &RedisDatastore{}
var _ datastore.ThreadSafeDatastore = &RedisDatastore{}

var ErrInvalidType = errors.New("redis datastore: invalid type error. this datastore only supports []byte values")

func NewExpiringDatastore(client *redis.Client, ttl time.Duration) (datastore.ThreadSafeDatastore, error) {
	return &RedisDatastore{
		client: client,
		ttl:    ttl,
	}, nil
}

func NewDatastore(client *redis.Client) (datastore.ThreadSafeDatastore, error) {
	return &RedisDatastore{
		client: client,
	}, nil
}

type RedisDatastore struct {
	mu     sync.Mutex
	client *redis.Client
	ttl    time.Duration
}

func (ds *RedisDatastore) Put(key datastore.Key, value interface{}) error {
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

func (ds *RedisDatastore) Get(key datastore.Key) (value interface{}, err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("GET", key.String()).Bytes()
}

func (ds *RedisDatastore) Has(key datastore.Key) (exists bool, err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("EXISTS", key.String()).Bool()
}

func (ds *RedisDatastore) Delete(key datastore.Key) (err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.client.Cmd("DEL", key.String()).Err
}

func (ds *RedisDatastore) Query(q query.Query) (query.Results, error) {
	return nil, errors.New("TODO implement query for redis datastore?")
}

func (ds *RedisDatastore) IsThreadSafe() {}
