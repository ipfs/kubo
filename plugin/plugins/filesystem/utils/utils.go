package fsutils

import "github.com/hugelgupf/p9/p9"

// WalkRef is used to generalize 9P `Walk` operations within filesystem boundries
// and allows for traversal across those boundaries if intended by the implementation
//
// The reference root node implementation links filesystems together using a parent/child relation
// sending the appropriate node back to the caller by using the linakge between nodes
// combined with inspection of a nodes current path
// it tries its best to avoid copying by modifying a node directly where possible
// falling back to derived copies when crossing filesystem boundaries during "movement"
type WalkRef interface {
	p9.File

	/* CheckWalk should make sure that the current reference adheres to the restrictions
	of 'walk(5)'
	In particular the reference must not be open for I/O, or otherwise already closed
	*/
	CheckWalk() error

	/* Fork allocates a new reference, derived from the existing reference
	Acting as the `newfid` construct mentioned in the documentation for the protocol
	A subset of the semantics will be noted here in a generalized way
	Please see 'walk(5)' for more information on the standard

	The returned reference node must "stand" beside the existing `WalkRef`
	Meaning the node must be "at"/contain the same location/path as the existing reference.

	The returned node must also adhear to 'walk(5)' `newfid` semantics
	Meaning that...
	`newfid` must be allowed to `Close` seperatley from the original reference
	`newfid`'s path may be modified during `Walk` without affecting the original `WalkRef`
	`Open` must flag all references within the same system, at the same path, as open
	etc. in compliance with 'walk(5)'
	*/
	Fork() (WalkRef, error)

	/* QID checks that the node's current path contains an existing file
	and returns the QID for it
	*/
	QID() (p9.QID, error)

	/* Step should return a reference that is tracking the result of
	the node's current-path + "name"

	It is valid to return a newly constructed reference or modify and return the existing reference
	as long as `QID` is ready to be called on the resulting node
	and resources are reaped where sensible within the fs implementation
	*/
	Step(name string) (WalkRef, error)

	/* Backtrack is the handler for `..` requests
	it is effectivley the inverse of `Step`
	if called on the root node, the node should return itself
	*/
	Backtrack() (parentRef WalkRef, err error)
}

// Walker implements the 9P `Walk` operation
func Walker(ref WalkRef, names []string) ([]p9.QID, p9.File, error) {
	// operations check
	err := ref.CheckWalk()
	if err != nil {
		return nil, nil, err
	}

	// no matter the outcome, we start with a `newfid`
	curRef, err := ref.Fork()
	if err != nil {
		return nil, nil, err
	}

	if shouldClone(names) {
		qid, err := ref.QID() // validate the node is "walkable"
		if err != nil {
			return nil, nil, err
		}
		return []p9.QID{qid}, curRef, nil
	}

	qids := make([]p9.QID, 0, len(names))

	for _, name := range names {
		switch name {
		default:
			// get ready to step forward; maybe across FS bounds
			curRef, err = curRef.Step(name)

		case ".":
			// don't prepare to move at all

		case "..":
			// get ready to step backwards; maybe across FS bounds
			curRef, err = curRef.Backtrack()
		}

		if err != nil {
			return qids, nil, err
		}

		// commit to the step
		qid, err := curRef.QID()
		if err != nil {
			return qids, nil, err
		}

		// set on success, we stepped forward
		qids = append(qids, qid)
	}

	return qids, curRef, nil
}

/* walk(5):
It is legal for `nwname` to be zero, in which case `newfid` will represent the same `file` as `fid`
and the `walk` will usually succeed; this is equivalent to walking to dot.
*/
func shouldClone(names []string) bool {
	switch len(names) {
	case 0: // truly empty path
		return true
	case 1: // self or empty but not nil
		pc := names[0]
		return pc == "." || pc == ""
	default:
		return false
	}
}
