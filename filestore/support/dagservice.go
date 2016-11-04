package filestore_support

import (
	"context"

	. "github.com/ipfs/go-ipfs/filestore"
	
	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
	dag "github.com/ipfs/go-ipfs/merkledag"
	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	cid "gx/ipfs/QmXUuRadqDq5BuFWzVU6VuKaSjTcNm1gNCtLvvP1TJCW4z/go-cid"	
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
	dsKey := b.CidToDsKey(c)
	_, dataObj, err := ds.fs.GetDirect(dsKey)
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
