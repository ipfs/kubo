package dagutils

import (
	"context"
	"errors"

	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
)

// Traversal provides methods to move through a DAG of IPLD nodes
// using an iterative algorithm and a handler for the user to perform
// some logic on each visited node. This enables the implementation of
// operations like reading and seeking inside of a UnixFS file DAG. In
// contrast with a recursive solution, the implicit path in the call
// stack is made explicit through the `path` variable.
//
// It exposes simple traversal operations like `Iterate` and  `Search`
// that are implemented through a series of simple move methods (`down`,
// `up`, `Right`) that modify its `path`. The `down` move method is the
// analogous of the recursive call and the one in charge of visiting
// the node (through the `VisitHandler`) and performing some user-defined
// logic.
//
// The position of the `Traversal` is defined through a `path` (slice)
// of nodes that form a parent-child relationship from the root to the
// current node being visited (at a depth marked by the `level`). Each
// of those nodes keep track of the link index its child in the path
// belongs to in order for the traversal algorithms to know not only
// what it the current nodes but which nodes (children) have already
// been visited (assuming always a left to right, zero to `len(Links())`,
// visiting order).
//
// TODO: Add a logic similar to `precalcNextBuf` to request several child
// nodes in advanced as node promises.
//
// TODO: Revisit the name. `Traverser`? (is "traverser" the correct noun?),
// `Iterator` (this would put to much emphasis on iterate whereas other
// traversal operations like search are supported), `Topology`?
//
// TODO: Encapsulate the position (`path`, `level`, `childIndex`)? It's
// a big part of the structure, and it should only be one of it, so it
// would just create a proxy for every move call, e.g., `dt.pos.down()`.
//
// TODO: Consider adding methods that would retrieve an attribute indexed
// by the current `level` to avoid including much indexing like
// `dt.path[dt.level]` in the code. Maybe another more strong refactoring
// that would allow to think of the current node in the `path` without
// concerning at what `level` it's in.
type Traversal struct {

	// Each member of the slice is the parent of the following member, from
	// the root to the current node, up until the position marked by `level`. The
	// slice may contain more elements past that point but they should be
	// ignored, every time the `level` increases the corresponding child node
	// is inserted (the slice is not truncated to leverage the already allocated
	// space). It should *always* have a length bigger than zero with the root
	// of the DAG at the first position (empty DAGs are not valid).
	path []ipld.Node

	// This slice has the index of the child each parent in `path` is pointing
	// to. Its current valid length is determined by `level`. The index in the
	// parent can be set past all of its child nodes (having a value equal to
	// `len(Links())`) to signal it has visited (or skipped) all of them. A
	// leaf node with no children and its index in zero would also comply with
	// this format.
	childIndex []uint
	// TODO: Revisit name, `childPosition`? Do not use the *link* term, put
	// emphasis in the child (the *what*, not the *how* we get it).

	// Depth of the node of the current position. It grows downwards, root is
	// level 0, its child level 1, and so on. It controls the effective length
	// of `path` and `childIndex`.
	//
	// A level of -1 signals the start case of a new `Traversal` that hasn't
	// moved yet (or its position has been reset, see `ResetPosition`).
	// Although this state is an invalid index to the slices, it allows to
	// centralize all the visit calls in the `down` move (starting at zero
	// would require a special visit case inside every traversal operation like
	// `Iterate()` and `Search`). This value should never be returned to after
	// the first `down` movement, moving up from the root should always return
	// `ErrUpOnRoot`.
	level int

	// Method called each time a node is arrived upon in a traversal (through
	// the `down()` movement, that is, when visiting it for the first time,
	// not when going back up). It is the main API to implement functionality
	// (like read and seek) on top of the `Traversal` structure.
	//
	// Its argument is the current `node` being visited. Any error it returns
	// (apart from the internal `errDownStopIteration`) will be forwarded to
	// the caller of the traversal operation (stopping it).
	//
	// Any of the exported methods of this API should be allowed to be called
	// from within this method.
	VisitHandler func(node ipld.Node) error

	// Flag to stop the current iteration, intended to be set by the user
	// inside `VisitHandler` to stop the current traversal operation.
	Stop bool
	// TODO: Use a function?

	// Attribute needed to fetch nodes, will become really useful once node
	// promises are implemented.
	ctx  context.Context
	serv ipld.NodeGetter

	// The CID of each child of each node in the `path` (indexed by
	// `level` and `childIndex`).
	childCIDs [][]*cid.Cid

	// NodePromises for child nodes requested for every node in the
	// `path` (as `childCIDs`, indexed by `level` and `childIndex`).
	promises [][]*ipld.NodePromise
	// TODO: Consider encapsulating in a single structure along `childCIDs`.

	// Cancel methods for every node in the `path` that had requested
	// child nodes through the `promises`.
	cancel []func()
}

// NewTraversal creates a new `Traversal` structure from a `root`
// IPLD node.
func NewTraversal(ctx context.Context, root ipld.Node, serv ipld.NodeGetter) *Traversal {
	return &Traversal{
		path:       []ipld.Node{root},
		childIndex: []uint{0},
		level:      -1,
		// Starting position, "on top" of the root node, see `level`.

		ctx:  ctx,
		serv: serv,

		childCIDs: make([][]*cid.Cid, 1),
		promises:  make([][]*ipld.NodePromise, 1),
		cancel:    make([]func(), 1),
		// Initial capacity of 1 (needed for the doubling capacity algorithm
		// of `extendPath`, it can't be zero).
	}
}

// ErrDownNoChild signals there are no more child nodes left to visit,
// the current child index is past the end of this node's links.
var ErrDownNoChild = errors.New("can't go down, no child available")

// ErrUpOnRoot signals the end of the DAG after returning to the root.
var ErrUpOnRoot = errors.New("can't go up, already on root")

// ErrRightNoChild signals the end of this parent child nodes.
var ErrRightNoChild = errors.New("can't move right, no more child nodes in this parent")

// errDownStopIteration signals the stop of the traversal operation.
var errDownStopIteration = errors.New("stop")

// ErrSearchNoVisitHandler signals the lack of a `VisitHandler` function.
var ErrSearchNoVisitHandler = errors.New("no visit handler specified for search")

// Iterate the DAG through the DFS pre-order traversal algorithm, going down
// as much as possible, then right to the other siblings, and then up (to go
// down again). The position is saved through iterations (and can be previously
// set in `Search`) allowing `Iterate` to be called repeatedly (after a stop)
// to continue the iteration. This function returns the errors received from
// `down` (generated either inside the `VisitHandler` call or any other errors
// while fetching the child nodes), the rest of the move errors are handled
// within the function and are not returned.
func (dt *Traversal) Iterate() error {

	// Iterate until either: the end of the DAG (`ErrUpOnRoot`), a stop
	// is requested (`errDownStopIteration`) or an error happens (while
	// going down).
	for {

		// First, go down as much as possible.
		for {
			err := dt.down()

			if err == ErrDownNoChild {
				break
				// Can't keep going down from this node, try to move right.
			}

			if err == errDownStopIteration {
				return nil
				// Stop requested, `errDownStopIteration` is just an internal
				// error to signal to stop, don't pass it along.
			}

			if err != nil {
				return err
				// `down()` is the only movement that can return *any* error
				// (different from the move errors).
			}
		}

		// Can't move down anymore through the current child index, go right
		// (increasing the index to the next child) to go down a different
		// path. If there are no more child nodes available, go back up.
		for {
			err := dt.Right()

			if err == nil {
				break
				// No error, it moved right. Try to go down again.
			}

			// It can't go right (`ErrRightNoChild`), try to move up.
			err = dt.up()

			if err != nil {
				return ErrUpOnRoot
				// Can't move up, must be on the root again. End of the DAG.
			}

			// Moved up, try right again.
		}

		// Moved right (after potentially many up moves), try going down again.
	}
}

// Search a specific node in a downwards manner. The `VisitHandler`
// should be used to select at each level through which child will the
// search continue (extending the `path` in that direction) or stop it
// (if the desired node has been found). The search always starts from
// the root. It modifies the position so it shouldn't be used in-between
// `Iterate` calls (it can be used to set the position *before* iterating).
//
// TODO: The search could be extended to search from the current position.
// (Is there something in the logic that would prevent it at the moment?)
func (dt *Traversal) Search() error {

	if dt.VisitHandler == nil {
		return ErrSearchNoVisitHandler
		// Although valid, there is no point in calling `Search` without
		// any extra logic, it would just go down to the leftmost leaf.
	}

	// Go down until it the desired node is found (that will be signaled
	// stopping the search with `errDownStopIteration`) or a leaf node is
	// reached (end of the DAG).
	for {
		err := dt.down()

		if err == errDownStopIteration {
			return nil
			// Found the node, `errDownStopIteration` is just an internal
			// error to signal to stop, don't pass it along.
		}

		if err == ErrDownNoChild {
			return nil
			// Can't keep going down from this node, either at a leaf node
			// or the `VisitHandler` has moved the child index past the
			// available index (probably because none indicated that the
			// target node could be down from there).
		}

		if err != nil {
			return err
			// `down()` is the only movement that can return *any* error
			// (different from the move errors).
		}
	}
	// TODO: Copied from the first part of `Iterate()` (although conceptually
	// different from it). Could this be encapsulated in a function? The way
	// the stop signal is handled it wouldn't seem very useful: the
	// `errDownStopIteration` needs to be processed at this level to return
	// (and stop the search, returning from another function here wouldn't
	// cause it to stop).
}

// Visit the current node, should only called from `down`. This is a wrapper
// function to `VisitHandler` to process the `Stop` signal and do other minor
// checks (taking this logic away from `down`).
func (dt *Traversal) visitNode() error {
	if dt.VisitHandler == nil {
		return nil
	}

	err := dt.VisitHandler(dt.path[dt.level])

	// Process the (potential) `Stop` signal (that the user can set
	// in the `VisitHandler`).
	if dt.Stop == true {
		dt.Stop = false

		if err == nil {
			err = errDownStopIteration
			// Set an artificial error (if `VisitHandler` didn't return one)
			// to stop the traversal.
		}
		// Else, `err != nil`, the iteration will be stopped as the `VisitHandler`
		// already returned an error, so return that instead of `errDownStopIteration`.
	}

	return err
}

// Go down one level in the DAG to the child pointed to by the index of the
// current node and perform some logic on it by calling the user-specified
// `VisitHandler`. This should always be the first move in any traversal
// operation (to visit the root node and move the `level` away from the
// negative value).
func (dt *Traversal) down() error {
	child, err := dt.fetchChild()
	if err != nil {
		return err
	}

	dt.extendPath(child)

	return dt.visitNode()
}

// Fetch the child from the current parent node in the `path`.
func (dt *Traversal) fetchChild() (ipld.Node, error) {
	if dt.level == -1 {
		// First time `down()` is called, `level` is -1, return the root node,
		// don't check available child nodes (as the `Traversal` is not
		// actually on any node just yet).
		return dt.path[0], nil
	}

	// Check if the next child to visit exists.
	childLinks := dt.path[dt.level].Links()
	if dt.childIndex[dt.level] >= uint(len(childLinks)) {
		return nil, ErrDownNoChild
	}
	// TODO: Can this check be included in `precalcNextBuf`?

	return dt.precalcNextBuf(dt.ctx)
}

// Increase the level and move down in the `path` to the fetched `child` node
// (which now becomes the current node). Fetch its links for future node
// requests. Allocate more space for the slices if needed.
func (dt *Traversal) extendPath(child ipld.Node) {

	dt.level++

	// Extend the slices if needed (doubling its capacity).
	if dt.level >= len(dt.path) {
		dt.path = append(dt.path, make([]ipld.Node, len(dt.path))...)
		dt.childIndex = append(dt.childIndex, make([]uint, len(dt.childIndex))...)
		dt.childCIDs = append(dt.childCIDs, make([][]*cid.Cid, len(dt.childCIDs))...)
		dt.promises = append(dt.promises, make([][]*ipld.NodePromise, len(dt.promises))...)
		dt.cancel = append(dt.cancel, make([]func(), len(dt.cancel))...)
		// TODO: Check the performance of these calls.
		// TODO: Could this be done in a generic function through reflection
		// (to get the type to for `make`).
	}

	dt.path[dt.level] = child
	dt.childIndex[dt.level] = 0
	// Always (re)set the child index to zero to start from the left.

	// If nodes were already requested at this `level` (but for
	// another node) cancel those requests (`ipld.NodePromise`).
	if dt.promises[dt.level] != nil {
		// TODO: Is this the correct check?

		dt.cancel[dt.level]()
		dt.promises[dt.level] = nil
	}

	dt.childCIDs[dt.level] = getLinkCids(child)
	dt.promises[dt.level] = make([]*ipld.NodePromise, len(dt.childCIDs[dt.level]))
	_, dt.cancel[dt.level] = context.WithCancel(dt.ctx)
	// TODO: Is this the correct context?
	// TODO: There's a "cascading" context, in the sense that one cancel seems
	// that should cancel all of the requests at all of the levels, check that.
	// (see `fctx`).
}

// Go up one level in the DAG. The only possible error this function can return
// is to signal it's already at the root and can't go up.
func (dt *Traversal) up() error {
	if dt.level < 1 {
		return ErrUpOnRoot
	}

	dt.level--

	return nil
}

// Right changes the child index of the current node to point to the next child
// (which may exist or may be the end of the available child nodes). This
// function doesn't actually move (i.e., it doesn't change the node we're
// positioned in), it just changes where are we pointing to next, it could be
// interpreted as "turn right".
//
// TODO: Clearly define the concept of position and differentiate it from the
// concept of "traveled path" (e.g., we're at *this* node but have already
// visited/covered all *these* other nodes); although keep in mind that
// `childIndex` is a complement of `path`, it's not independent of it (those
// are the indexes *of* the nodes in the `path`). So, the question is, does
// "turning rigth" changes the current position? Or state? Or traveled path?
func (dt *Traversal) Right() error {

	// Move the child index up until the (nonexistent) position past all of the
	// child nodes (`len(childLinks)`).
	childLinks := dt.path[dt.level].Links()
	if dt.childIndex[dt.level]+1 <= uint(len(childLinks)) {
		dt.childIndex[dt.level]++
	}

	if dt.childIndex[dt.level] == uint(len(childLinks)) {
		return ErrRightNoChild
		// At the end of the available children of the current node, signal it.
	}

	return nil
}

// ResetPosition sets the position of the `Traversal` back to the
// original state defined in `NewTraversal`. As the three position
// attributes (`path`, `level`, `childIndex`) are the only state of
// structure, resetting the position is effectively the equivalent
// of recreating (with `NewTraversal`) the entire structure.
func (dt *Traversal) ResetPosition() {
	dt.level = -1
	// As `level` controls the usage of `path` and `childIndex`
	// setting its value is enough to reset the position. This also
	// allows to take advantage of the already allocated space in the
	// slices.
}

// ChildIndex returns the index of the child pointed to by the current
// parent node.
func (dt *Traversal) ChildIndex() uint {
	return dt.childIndex[dt.level]
}

// TODO: Give more visibility to this constant.
const preloadSize = 10

func (dt *Traversal) preload(ctx context.Context, beg uint) {
	end := beg + preloadSize
	if end >= uint(len(dt.childCIDs[dt.level])) {
		end = uint(len(dt.childCIDs[dt.level]))
	}

	copy(dt.promises[dt.level][beg:], ipld.GetNodes(ctx, dt.serv, dt.childCIDs[dt.level][beg:end]))
}

// precalcNextBuf follows the next link in line and loads it from the
// DAGService, setting the next buffer to read from
//
// TODO: Where is this `ctx` coming from?
func (dt *Traversal) precalcNextBuf(ctx context.Context) (ipld.Node, error) {

	// If we drop to <= preloadSize/2 preloading nodes, preload the next 10.
	for i := dt.childIndex[dt.level]; i < dt.childIndex[dt.level]+preloadSize/2 && i < uint(len(dt.promises[dt.level])); i++ {
		// TODO: check if canceled.
		if dt.promises[dt.level][i] == nil {
			dt.preload(ctx, i)
			break
		}
	}

	// Fetch the actual node (this is the blocking part of the mechanism)
	// and invalidate the promise.
	nxt, err := dt.promises[dt.level][dt.childIndex[dt.level]].Get(ctx)
	dt.promises[dt.level][dt.childIndex[dt.level]] = nil
	// TODO: Great example of why `level` should go.

	switch err {
	case nil:
	case context.DeadlineExceeded, context.Canceled:
		err = ctx.Err()
		if err != nil {
			return nil, ctx.Err()
		}
		// In this case, the context used to *preload* the node has been canceled.
		// We need to retry the load with our context and we might as
		// well preload some extra nodes while we're at it.
		//
		// Note: When using `Read`, this code will never execute as
		// `Read` will use the global context. It only runs if the user
		// explicitly reads with a custom context (e.g., by calling
		// `CtxReadFull`).
		dt.preload(ctx, dt.childIndex[dt.level])
		nxt, err = dt.promises[dt.level][dt.childIndex[dt.level]].Get(ctx)
		dt.promises[dt.level][dt.childIndex[dt.level]] = nil
		// TODO: Same code as before.
		if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	return nxt, nil
}

func getLinkCids(node ipld.Node) []*cid.Cid {
	links := node.Links()
	out := make([]*cid.Cid, 0, len(links))

	for _, l := range links {
		out = append(out, l.Cid)
	}
	return out
}

// TODO: Move to another package in the `go-ipld-format` repository.
