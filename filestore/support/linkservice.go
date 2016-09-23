package filestore_support

import (
	. "github.com/ipfs/go-ipfs/filestore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

func NewLinkService(fs *Datastore) dag.LinkService {
	return &linkservice{fs}
}

type linkservice struct {
	fs *Datastore
}

func GetLinks(dataObj *DataObj) ([]*dag.Link, error) {
	res, err := dag.DecodeProtobuf(dataObj.Data)
	if err != nil {
		return nil, err
	}
	return res.Links, nil
}

func (ls *linkservice) Get(cid *cid.Cid) ([]*dag.Link, error) {
	dsKey := key.Key(cid.Hash()).DsKey()
	_, dataObj, err := ls.fs.GetDirect(dsKey)
	if err == ds.ErrNotFound {
		return nil, dag.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return GetLinks(dataObj)
}
