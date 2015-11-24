package redis

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/fzzy/radix/redis"
	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"

	dstest "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/test"
)

const RedisEnv = "REDIS_DATASTORE_TEST_HOST"

func TestPutGetBytes(t *testing.T) {
	client := clientOrAbort(t)
	ds, err := NewDatastore(client)
	if err != nil {
		t.Fatal(err)
	}
	key, val := datastore.NewKey("foo"), []byte("bar")
	dstest.Nil(ds.Put(key, val), t)
	v, err := ds.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(v.([]byte), val) != 0 {
		t.Fail()
	}
}

func TestHasBytes(t *testing.T) {
	client := clientOrAbort(t)
	ds, err := NewDatastore(client)
	if err != nil {
		t.Fatal(err)
	}
	key, val := datastore.NewKey("foo"), []byte("bar")
	has, err := ds.Has(key)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fail()
	}

	dstest.Nil(ds.Put(key, val), t)
	hasAfterPut, err := ds.Has(key)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAfterPut {
		t.Fail()
	}
}

func TestDelete(t *testing.T) {
	client := clientOrAbort(t)
	ds, err := NewDatastore(client)
	if err != nil {
		t.Fatal(err)
	}
	key, val := datastore.NewKey("foo"), []byte("bar")
	dstest.Nil(ds.Put(key, val), t)
	dstest.Nil(ds.Delete(key), t)

	hasAfterDelete, err := ds.Has(key)
	if err != nil {
		t.Fatal(err)
	}
	if hasAfterDelete {
		t.Fail()
	}
}

func TestExpiry(t *testing.T) {
	ttl := 1 * time.Second
	client := clientOrAbort(t)
	ds, err := NewExpiringDatastore(client, ttl)
	if err != nil {
		t.Fatal(err)
	}
	key, val := datastore.NewKey("foo"), []byte("bar")
	dstest.Nil(ds.Put(key, val), t)
	time.Sleep(ttl + 1*time.Second)
	dstest.Nil(ds.Delete(key), t)

	hasAfterExpiration, err := ds.Has(key)
	if err != nil {
		t.Fatal(err)
	}
	if hasAfterExpiration {
		t.Fail()
	}
}

func clientOrAbort(t *testing.T) *redis.Client {
	c, err := redis.Dial("tcp", os.Getenv(RedisEnv))
	if err != nil {
		t.Log("could not connect to a redis instance")
		t.SkipNow()
	}
	if err := c.Cmd("FLUSHALL").Err; err != nil {
		t.Fatal(err)
	}
	return c
}
