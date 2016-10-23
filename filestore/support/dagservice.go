package filestore_support

import (
	"context"

	. "github.com/ipfs/go-ipfs/filestore"

	dag "github.com/ipfs/go-ipfs/merkledag"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
)

func NewDAGService(fs *Datastore, ds dag.DAGService) dag.DAGService {
	return &dagService{fs, ds}
}

type dagService struct {
	fs *Datastore
	dag.DAGService
}

func GetLinks(dataObj *DataObj) ([]*node.Link, error) {
	res, err := dag.DecodeProtobuf(dataObj.Data)
	if err != nil {
		return nil, err
	}
	return res.Links(), nil
}

func (ds *dagService) GetLinks(ctx context.Context, c *cid.Cid) ([]*node.Link, error) {
	dsKey := dshelp.CidToDsKey(c)
	key := NewDbKey(dsKey.String(), "", -1, nil)
	_, dataObj, err := ds.fs.GetDirect(key)
	if err != nil {
		return ds.DAGService.GetLinks(ctx, c)
	}
	return GetLinks(dataObj)
}

func (ds *dagService) GetOfflineLinkService() dag.LinkService {
	ds2 := ds.DAGService.GetOfflineLinkService()
	if ds != ds2 {
		return NewDAGService(ds.fs, ds.DAGService)
	} else {
		return ds2
	}
}
