package namesys

import (
	"encoding/binary"

	path "github.com/ipfs/go-ipfs/path"

	ds "gx/ipfs/QmPpegoMqhAEqjncrzArm7KVWAkCm78rqL2DPuNjhPrshg/go-datastore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	dshelp "gx/ipfs/QmdQTPWduSeyveSxeCAte33M592isSW5Z979g81aJphrgn/go-ipfs-ds-help"
)

// RepubCachePut stores the record value and sequence number for the peer ID in the republisher cache
func RepubCachePut(dstore ds.Datastore, id peer.ID, seqnum uint64, val path.Path) error {
	log.Debugf("RepubCachePut %v (%d) %s", id, seqnum, val)
	rec := marshallRepubData(seqnum, val)
	k := repubKeyForID(id)
	return dstore.Put(dshelp.NewKeyFromBinary([]byte(k)), rec)
}

// RepubCacheGet fetches the record value and sequence number for the peer ID from republisher cache
func RepubCacheGet(dstore ds.Datastore, id peer.ID) (uint64, path.Path, error) {
	k := repubKeyForID(id)
	ival, err := dstore.Get(dshelp.NewKeyFromBinary([]byte(k)))
	if err != nil {
		return 0, "", err
	}

	val := ival.([]byte)
	return unmarshallRepubData(val)
}

func marshallRepubData(seqnum uint64, val path.Path) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, seqnum)
	return append(b, []byte(val)...)
}

func unmarshallRepubData(data []byte) (uint64, path.Path, error) {
	seqnum := binary.LittleEndian.Uint64(data[0:8])
	pstr := string(data[8:])
	p, err := path.ParsePath(pstr)
	return seqnum, p, err
}

func repubKeyForID(id peer.ID) string {
	return "/repub/" + string(id)
}
