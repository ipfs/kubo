package filestore_support

import (
	key "github.com/ipfs/go-ipfs/blocks/key"
	. "github.com/ipfs/go-ipfs/filestore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ds "gx/ipfs/QmTxLSvdhwg68WJimdS6icLPhZi28aTp6b7uihC2Yb47Xk/go-datastore"
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

func (ls *linkservice) Get(key key.Key) ([]*dag.Link, error) {
	dsKey := key.DsKey()
	_, dataObj, err := ls.fs.GetDirect(dsKey)
	if err == ds.ErrNotFound {
		return nil, dag.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return GetLinks(dataObj)
}
