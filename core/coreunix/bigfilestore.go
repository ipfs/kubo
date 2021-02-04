package coreunix

import (
	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
	dsns "github.com/ipfs/go-datastore/namespace"
	dshelp "github.com/ipfs/go-ipfs-ds-help"
)

type BigFileStore struct {
	dstore ds.Datastore
}

// bigFilePrefix namespaces big file datastores
var bigFilePrefix = ds.NewKey("bigfiles")

// NewBigFileStore creates a new bifFileStore
func NewBigFileStore(dstore ds.Datastore) *BigFileStore {
	return &BigFileStore{
		dstore: dsns.Wrap(dstore, bigFilePrefix),
	}
}

func (b *BigFileStore) Put(streamCid cid.Cid, chunks []*ChunkingManifestChunk) error {
	chunkData, err := serializeChunks(chunks)
	if err != nil {
		return err
	}

	dsk := dshelp.CidToDsKey(streamCid)
	return b.dstore.Put(dsk, chunkData)
}

func (b *BigFileStore) Get(streamCid cid.Cid) ([]*ChunkingManifestChunk, error) {
	data, err := b.dstore.Get(dshelp.CidToDsKey(streamCid))
	if err != nil {
		return nil, err
	}

	return deserializeChunks(data)
}
