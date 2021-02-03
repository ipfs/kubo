package coreunix

import (
	"context"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
)

type ChunkingManifest struct {
	StreamCid  cid.Cid
	ChunkedCid cid.Cid
	Chunks     []*ChunkingManifestChunk
}

type ChunkingManifestChunk struct {
	ChunkCid cid.Cid
	Offset   uint64
	Size     uint64
}

func extractChunkingManifest(ctx context.Context, dagSvc ipld.DAGService, chunkedFileCid cid.Cid) (*ChunkingManifest, error) {
	getLinks := dag.GetLinksWithDAG(dagSvc)
	chunking := &ChunkingManifest{
		// TODO: compute and set stream cid (aka "SID")
		ChunkedCid: chunkedFileCid,
	}
	var verr error
	var offset uint64
	visitor := func(d cid.Cid) bool {
		// if block is not raw, continue
		if d.Type() != cid.Raw {
			return true
		}
		// otherwise,  append the chunk to the manifest
		dn, err := dagSvc.Get(ctx, d)
		if err != nil {
			verr = err
			return false
		}
		sz, err := dn.Size()
		if err != nil {
			verr = err
			return false
		}
		chunking.Chunks = append(chunking.Chunks,
			&ChunkingManifestChunk{
				ChunkCid: d,
				Size:     sz,
				Offset:   offset,
			})
		offset += sz
		return true
	}
	if err := dag.Walk(context.TODO(), getLinks, chunkedFileCid, visitor); err != nil {
		return nil, err
	}
	if verr != nil {
		return nil, verr
	}
	return chunking, nil
}
