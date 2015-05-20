package s3datastore

import (
	"encoding/hex"
	"errors"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"
	datastore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	query "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var _ datastore.ThreadSafeDatastore = &S3Datastore{}

var errTODO = errors.New("TODO")

var ErrInvalidType = errors.New("s3 datastore: invalid type error")

type S3Datastore struct {
	Client *s3.S3
	Bucket string
	ACL    s3.ACL
}

func (ds *S3Datastore) encode(key datastore.Key) string {
	return hex.EncodeToString(key.Bytes())
}

func (ds *S3Datastore) decode(raw string) (datastore.Key, bool) {
	k, err := hex.DecodeString(raw)
	if err != nil {
		return datastore.Key{}, false
	}
	return datastore.NewKey(string(k)), true
}

func (ds *S3Datastore) Put(key datastore.Key, value interface{}) (err error) {
	data, ok := value.([]byte)
	if !ok {
		return ErrInvalidType
	}
	// TODO extract s3 options

	k := ds.encode(key)
	acl := ds.ACL
	if acl == "" {
		acl = s3.Private
	}
	return ds.Client.Bucket(ds.Bucket).Put(k, data, "application/protobuf", acl, s3.Options{})
}

func (ds *S3Datastore) Get(key datastore.Key) (value interface{}, err error) {
	k := ds.encode(key)
	return ds.Client.Bucket(ds.Bucket).Get(k)
}

func (ds *S3Datastore) Has(key datastore.Key) (exists bool, err error) {
	k := ds.encode(key)
	return ds.Client.Bucket(ds.Bucket).Exists(k)
}

func (ds *S3Datastore) Delete(key datastore.Key) (err error) {
	k := ds.encode(key)
	return ds.Client.Bucket(ds.Bucket).Del(k)
}

func (ds *S3Datastore) Query(q query.Query) (query.Results, error) {
	return nil, errors.New("TODO implement query for s3 datastore?")
}

func (ds *S3Datastore) IsThreadSafe() {}
