package pin

import (
	cid "gx/ipfs/QmNp85zy9RLrQ5oQD4hPyS39ezrrXpcaa7R4Y9kxdWQLLQ/go-cid"
)

// Source is structure describing source of the pin
type Source struct {
	// Get if a function that will be called to get the pins for GCing
	Get func() ([]*cid.Cid, error)
	// Strict makes the GC fail if some objects can't be fetched during
	// recursive traversal of the graph
	Strict bool
	// Direct marks the pinned object as the final object in the traversal
	Direct bool
	// Internal marks the pin source which recursive enumeration should be
	// terminated by a direct pin
	Internal bool
}

// SortValue is sort order for the GC
func (p Source) SortValue() int {
	v := 0
	if !p.Strict {
		v |= 1 << 0
	}
	if p.Direct {
		v |= 1 << 1
	}
	if p.Internal {
		v |= 1 << 2
	}
	return v
}
