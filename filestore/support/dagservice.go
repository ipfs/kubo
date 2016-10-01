package filestore_support

import (
	. "github.com/ipfs/go-ipfs/filestore"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"
	//ds "gx/ipfs/QmbzuUusHqaLLoNTDEVLcSF6vZDHZDLPC7p4bztRvvkXxU/go-datastore"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
	cid "gx/ipfs/QmfSc2xehWmWLnwwYR91Y8QF4xdASypTFVknutoKQS3GHp/go-cid"
)

func NewDAGService(fs *Datastore, ds dag.DAGService) dag.DAGService {
	return &dagService{fs, ds}
}

type dagService struct {
	fs *Datastore
	dag.DAGService
}

func GetLinks(dataObj *DataObj) ([]*dag.Link, error) {
	res, err := dag.DecodeProtobuf(dataObj.Data)
	if err != nil {
		return nil, err
	}
	return res.Links, nil
}

func (ds *dagService) GetLinks(ctx context.Context, cid *cid.Cid) ([]*dag.Link, error) {
	dsKey := key.Key(cid.Hash()).DsKey()
	_, dataObj, err := ds.fs.GetDirect(dsKey)
	if err != nil {
		return ds.DAGService.GetLinks(ctx, cid)
	}
	return GetLinks(dataObj)
}

func (ds *dagService) GetOfflineLinkService() dag.LinkService {
	ds2 := ds.DAGService.GetOfflineLinkService()
	if (ds != ds2) {
		return NewDAGService(ds.fs, ds.DAGService)
	} else {
		return ds2
	}
}
