package dht

import (
	"encoding/json"
	"time"

	peer "github.com/jbenet/go-ipfs/peer"
)

type connDiagInfo struct {
	Latency time.Duration
	Id peer.ID
}

type diagInfo struct {
	Id peer.ID
	Connections []connDiagInfo
	Keys []string
	LifeSpan time.Duration
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
	di.Id = dht.self.ID
	di.LifeSpan = time.Since(dht.birth)
	di.Keys = nil // Currently no way to query datastore

	for _,p := range dht.routes[0].listpeers() {
		di.Connections = append(di.Connections, connDiagInfo{p.GetDistance(), p.ID})
	}
	return di
}
