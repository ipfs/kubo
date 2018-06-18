package recpinset

import (
	cid "gx/ipfs/QmapdYm1b22Frv3k17fqrBYTFRxwiaVJkB299Mfn33edeB/go-cid"
)

// Set stores a set of recursive pins (Cid,MaxDepth tuples), indexed by Cid.
// A MaxDepth of -1 means fully no-limit recursive pin.
type Set struct {
	set map[string]int
}

// RecPin represents a recursive Pin, that is, a CID and a MaxDepth that
// limits the depth to which the CID is pinned. A negative MaxDepth represents
// a unlimited-depth pin.
type RecPin struct {
	Cid      *cid.Cid
	MaxDepth int
}

// New initializes and returns a new Set.
func New() *Set {
	return &Set{set: make(map[string]int)}
}

// Add puts a Cid and the maxDepth associated to it in the Set.
func (s *Set) Add(c *cid.Cid, maxDepth int) {
	s.set[string(c.Bytes())] = maxDepth
}

// Has returns if the Set contains a given Cid.
func (s *Set) Has(c *cid.Cid) bool {
	_, ok := s.set[string(c.Bytes())]
	return ok
}

// Get returns the RecPin associated to the given Cid.
func (s *Set) Get(c *cid.Cid) (*RecPin, bool) {
	md, ok := s.set[string(c.Bytes())]
	if !ok {
		return nil, false
	}

	return &RecPin{c, md}, ok
}

// MaxDepth returns the MaxDepth associated to the given Cid.
func (s *Set) MaxDepth(c *cid.Cid) (int, bool) {
	md, ok := s.set[string(c.Bytes())]
	return md, ok
}

// Remove deletes a Cid from the Set.
func (s *Set) Remove(c *cid.Cid) {
	delete(s.set, string(c.Bytes()))
}

// Len returns how many elements the Set has.
func (s *Set) Len() int {
	return len(s.set)
}

// Keys returns the Cids in the set.
func (s *Set) Keys() []*cid.Cid {
	out := make([]*cid.Cid, 0, len(s.set))
	for k := range s.set {
		c, _ := cid.Cast([]byte(k))
		out = append(out, c)
	}
	return out
}

// RecPins returns all the Cid+MaxDepths stored in the set,
// in a slice of RecPins.
func (s *Set) RecPins() []*RecPin {
	out := make([]*RecPin, 0, len(s.set))
	for k, v := range s.set {
		c, _ := cid.Cast([]byte(k))
		out = append(out, &RecPin{c, v})
	}
	return out
}

// Visit adds a cid and maxDepth to the set if:
// - it is not already in the set
// - if it's in the set but the new maxDepth is greater than
// the existing.
// It returns true if the RecPin has been added.
func (s *Set) Visit(c *cid.Cid, maxDepth int) bool {
	curMaxDepth, ok := s.set[string(c.Bytes())]

	if !ok || IsDeeper(maxDepth, curMaxDepth) {
		s.Add(c, maxDepth)
		return true
	}

	return false
}

// ForEach allows to run a custom function on each
// Cid in the set.
func (s *Set) ForEach(f func(c *cid.Cid, maxDepth int) error) error {
	for cs, md := range s.set {
		c, _ := cid.Cast([]byte(cs))
		err := f(c, md)
		if err != nil {
			return err
		}
	}
	return nil
}

// Returns true if d1 is deeper than d2
// Takes into account that -1 is deeper than anything.
func IsDeeper(d1, d2 int) bool {
	// if d2 is negative, nothing is deeper: no
	if d2 < 0 {
		return false
	}

	// d2 is >= 0 here.

	// if d1 is negative, yes
	if d1 < 0 {
		return true
	}

	// if d1 > d2, yes
	return d1 > d2
}
