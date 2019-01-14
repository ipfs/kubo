package iface

import (
	ipld "gx/ipfs/QmRL22E4paat7ky7vx9MLpR97JHHbFPrg3ytFQw6qp1y1s/go-ipld-format"
)

// APIDagService extends ipld.DAGService
type APIDagService interface {
	ipld.DAGService

	// Pinning returns special NodeAdder which recursively pins added nodes
	Pinning() ipld.NodeAdder
}
