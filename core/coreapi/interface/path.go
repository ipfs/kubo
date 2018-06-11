package iface

import (
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
)

// Path is a generic wrapper for paths used in the API. A path can be resolved
// to a CID using one of Resolve functions in the API.
type Path interface {
	// String returns the path as a string.
	String() string
	// Cid returns cid referred to by path
	Cid() *cid.Cid
	// Root returns cid of root path
	Root() *cid.Cid
	// Resolved returns whether path has been fully resolved
	Resolved() bool
}
