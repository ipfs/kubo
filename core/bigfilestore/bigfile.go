package bigfilestore

import (
	"bytes"
	"context"
	"io"
	"strings"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	multibase "github.com/multiformats/go-multibase"
	"github.com/polydawn/refmt/cbor"
	"github.com/polydawn/refmt/obj/atlas"
)

type ChunkingManifest struct {
	ChunkedCid cid.Cid
	Chunks     []*ChunkingManifestChunk
}

type ChunkingManifestChunk struct {
	ChunkCid cid.Cid
	Offset   uint64
	Size     uint64
}

func ExtractChunkingManifest(ctx context.Context, dagSvc ipld.DAGService, chunkedFileCid cid.Cid) (*ChunkingManifest, error) {
	getLinks := dag.GetLinksWithDAG(dagSvc)
	chunking := &ChunkingManifest{
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

var chunkAtl atlas.Atlas

func init() {
	chunkAtl = atlas.MustBuild(
		atlas.BuildEntry(ChunkingManifestChunk{}).StructMap().
			AddField("ChunkCid", atlas.StructMapEntry{SerialName: "cid"}).
			AddField("Offset", atlas.StructMapEntry{SerialName: "offset"}).
			AddField("Size", atlas.StructMapEntry{SerialName: "size"}).
			Complete(),
		atlas.BuildEntry(cid.Cid{}).Transform().
			TransformMarshal(atlas.MakeMarshalTransformFunc(func(live cid.Cid) ([]byte, error) { return live.MarshalBinary() })).
			TransformUnmarshal(atlas.MakeUnmarshalTransformFunc(func(serializable []byte) (cid.Cid, error) {
				c := cid.Cid{}
				err := c.UnmarshalBinary(serializable)
				if err != nil {
					return cid.Cid{}, err
				}
				return c, nil
			})).Complete(),
	)
}

func (c ChunkingManifestChunk) Serialize() ([]byte, error) {
	b, err := cbor.MarshalAtlased(c, chunkAtl)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (c *ChunkingManifestChunk) Deserialize(data []byte) error {
	return cbor.UnmarshalAtlased(cbor.DecodeOptions{}, data, c, chunkAtl)
}

func serializeChunks(chunks []*ChunkingManifestChunk) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	var b strings.Builder
	for i := range chunks {
		cborChunk, err := chunks[i].Serialize()
		if err != nil {
			return nil, err
		}
		encData, err := multibase.Encode(multibase.Base64url, cborChunk)
		if err != nil {
			// programming error; using unsupported encoding
			panic(err.Error())
		}
		b.WriteString(encData)
		b.WriteString(" ")
	}
	dataBlock := b.String()
	return []byte(dataBlock[:len(dataBlock)-1]), nil
}

func deserializeChunks(data []byte) ([]*ChunkingManifestChunk, error) {
	var chunks []*ChunkingManifestChunk
	b := bytes.NewBuffer(data)

	var done bool
	for !done {
		encStr, err := b.ReadString(byte(' '))
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			if encStr == "" {
				break
			}
			done = true
		} else {
			encStr = encStr[:len(encStr)-1]
		}
		_, cborChunk, err := multibase.Decode(encStr)
		if err != nil {
			return nil, err
		}

		chunk := &ChunkingManifestChunk{}
		err = chunk.Deserialize(cborChunk)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, chunk)
	}
	return chunks, nil
}
