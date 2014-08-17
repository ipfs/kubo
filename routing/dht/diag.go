package dht

import (
	"encoding/json"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
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
	di.CodeVersion = "github.com/jbenet/go-ipfs"
	di.ID = dht.self.ID
	di.LifeSpan = time.Since(dht.birth)
	di.Keys = nil // Currently no way to query datastore

	for _, p := range dht.routingTables[0].ListPeers() {
		di.Connections = append(di.Connections, connDiagInfo{p.GetLatency(), p.ID})
	}
	return di
}
