package helpers

import (
	dag "github.com/ipfs/go-ipfs/merkledag"
	"github.com/ipfs/go-ipfs/pin"
	u "github.com/ipfs/go-ipfs/util"
)

type BlockCB func(u.Key, bool) error

var nilFunc BlockCB = func(_ u.Key, _ bool) error { return nil }

// DagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type DagBuilderHelper struct {
	dserv    dag.DAGService
	mp       pin.ManualPinner
	in       <-chan []byte
	nextData []byte // the next item to return.
	maxlinks int
	bcb      BlockCB
}

type DagBuilderParams struct {
	// Maximum number of links per intermediate node
	Maxlinks int

	// DAGService to write blocks to (required)
	Dagserv dag.DAGService

	// Callback for each block added
	BlockCB BlockCB
}

// Generate a new DagBuilderHelper from the given params, using 'in' as a
// data source
func (dbp *DagBuilderParams) New(in <-chan []byte) *DagBuilderHelper {
	bcb := dbp.BlockCB
	if bcb == nil {
		bcb = nilFunc
	}

	return &DagBuilderHelper{
		dserv:    dbp.Dagserv,
		in:       in,
		maxlinks: dbp.Maxlinks,
		bcb:      bcb,
	}
}

// prepareNext consumes the next item from the channel and puts it
// in the nextData field. it is idempotent-- if nextData is full
// it will do nothing.
//
// i realized that building the dag becomes _a lot_ easier if we can
// "peek" the "are done yet?" (i.e. not consume it from the channel)
func (db *DagBuilderHelper) prepareNext() {
	if db.in == nil {
		// if our input is nil, there is "nothing to do". we're done.
		// as if there was no data at all. (a sort of zero-value)
		return
	}

	// if we already have data waiting to be consumed, we're ready.
	if db.nextData != nil {
		return
	}

	// if it's closed, nextData will be correctly set to nil, signaling
	// that we're done consuming from the channel.
	db.nextData = <-db.in
}

// Done returns whether or not we're done consuming the incoming data.
func (db *DagBuilderHelper) Done() bool {
	// ensure we have an accurate perspective on data
	// as `done` this may be called before `next`.
	db.prepareNext() // idempotent
	return db.nextData == nil
}

// Next returns the next chunk of data to be inserted into the dag
// if it returns nil, that signifies that the stream is at an end, and
// that the current building operation should finish
func (db *DagBuilderHelper) Next() []byte {
	db.prepareNext() // idempotent
	d := db.nextData
	db.nextData = nil // signal we've consumed it
	return d
}

// GetDagServ returns the dagservice object this Helper is using
func (db *DagBuilderHelper) GetDagServ() dag.DAGService {
	return db.dserv
}

// FillNodeLayer will add datanodes as children to the give node until
// at most db.indirSize ndoes are added
//
// warning: **children** pinned indirectly, but input node IS NOT pinned.
func (db *DagBuilderHelper) FillNodeLayer(node *UnixfsNode) error {

	// while we have room AND we're not done
	for node.NumChildren() < db.maxlinks && !db.Done() {
		child := NewUnixfsBlock()

		if err := db.FillNodeWithData(child); err != nil {
			return err
		}

		if err := node.AddChild(child, db); err != nil {
			return err
		}
	}

	return nil
}

func (db *DagBuilderHelper) FillNodeWithData(node *UnixfsNode) error {
	data := db.Next()
	if data == nil { // we're done!
		return nil
	}

	if len(data) > BlockSizeLimit {
		return ErrSizeLimitExceeded
	}

	node.SetData(data)
	return nil
}

func (db *DagBuilderHelper) Add(node *UnixfsNode) (*dag.Node, error) {
	dn, err := node.GetDagNode()
	if err != nil {
		return nil, err
	}

	key, err := db.dserv.Add(dn)
	if err != nil {
		return nil, err
	}

	// block callback
	err = db.bcb(key, true)
	if err != nil {
		return nil, err
	}

	return dn, nil
}

func (db *DagBuilderHelper) Maxlinks() int {
	return db.maxlinks
}
