package dht

import (
	"encoding/json"
	"time"

	peer "gx/ipfs/QmbyvM8zRFDkbFdYyt1MnevUMJ62SiSGbfDFZ3Z8nkrzr4/go-libp2p-peer"
)

type connDiagInfo struct {
	Latency time.Duration
	ID      peer.ID
}

type diagInfo struct {
	ID          peer.ID
	Connections []connDiagInfo
	Keys        []string
	LifeSpan    time.Duration
	CodeVersion string
}

func (di *diagInfo) Marshal() []byte {
	b, err := json.Marshal(di)
	if err != nil {
		panic(err)
	}
	//TODO: also consider compressing this. There will be a lot of these
	return b
}

func (dht *IpfsDHT) getDiagInfo() *diagInfo {
	di := new(diagInfo)
	di.CodeVersion = "github.com/ipfs/go-ipfs"
	di.ID = dht.self
	di.LifeSpan = time.Since(dht.birth)
	di.Keys = nil // Currently no way to query datastore

	for _, p := range dht.routingTable.ListPeers() {
		d := connDiagInfo{dht.peerstore.LatencyEWMA(p), p}
		di.Connections = append(di.Connections, d)
	}
	return di
}
