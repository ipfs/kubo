package s3datastore

import (
	"errors"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"
	datastore "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	query "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
)

var _ datastore.Datastore = &S3Datastore{}

var errTODO = errors.New("TODO")

var ErrInvalidType = errors.New("s3 datastore: invalid type error")

type S3Datastore struct {
	Client *s3.S3
	Bucket string
}

func (ds *S3Datastore) Put(key datastore.Key, value interface{}) (err error) {
	data, ok := value.([]byte)
	if !ok {
		return ErrInvalidType
	}
	// TODO extract perms and s3 options
	return ds.Client.Bucket(ds.Bucket).Put(key.String(), data, "application/protobuf", s3.PublicRead, s3.Options{})
}

func (ds *S3Datastore) Get(key datastore.Key) (value interface{}, err error) {
	return ds.Client.Bucket(ds.Bucket).Get(key.String())
}

func (ds *S3Datastore) Has(key datastore.Key) (exists bool, err error) {
	return ds.Client.Bucket(ds.Bucket).Exists(key.String())
}

func (ds *S3Datastore) Delete(key datastore.Key) (err error) {
	return ds.Client.Bucket(ds.Bucket).Del(key.String())
}

func (ds *S3Datastore) Query(q query.Query) (query.Results, error) {
	return nil, errors.New("TODO implement query for s3 datastore?")
}
