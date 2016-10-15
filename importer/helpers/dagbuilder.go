package helpers

import (
	"github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"

	node "gx/ipfs/QmZx42H5khbVQhV5odp66TApShV4XCujYazcvYduZ4TroB/go-ipld-node"
)

// DagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type DagBuilderHelper struct {
	dserv     dag.DAGService
	spl       chunk.Splitter
	recvdErr  error
	rawLeaves bool
	nextData  []byte // the next item to return.
	maxlinks  int
	batch     *dag.Batch
}

type DagBuilderParams struct {
	// Maximum number of links per intermediate node
	Maxlinks int

	// RawLeaves signifies that the importer should use raw ipld nodes as leaves
	// instead of using the unixfs TRaw type
	RawLeaves bool

	// DAGService to write blocks to (required)
	Dagserv dag.DAGService
}

// Generate a new DagBuilderHelper from the given params, which data source comes
// from chunks object
func (dbp *DagBuilderParams) New(spl chunk.Splitter) *DagBuilderHelper {
	return &DagBuilderHelper{
		dserv:     dbp.Dagserv,
		spl:       spl,
		rawLeaves: dbp.RawLeaves,
		maxlinks:  dbp.Maxlinks,
		batch:     dbp.Dagserv.Batch(),
	}
}

// prepareNext consumes the next item from the splitter and puts it
// in the nextData field. it is idempotent-- if nextData is full
// it will do nothing.
func (db *DagBuilderHelper) prepareNext() {
	// if we already have data waiting to be consumed, we're ready
	if db.nextData != nil {
		return
	}

	// TODO: handle err (which wasn't handled either when the splitter was channeled)
	db.nextData, _ = db.spl.NextBytes()
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
func (db *DagBuilderHelper) FillNodeLayer(node *UnixfsNode) error {

	// while we have room AND we're not done
	for node.NumChildren() < db.maxlinks && !db.Done() {
		child, err := db.GetNextDataNode()
		if err != nil {
			return err
		}

		if err := node.AddChild(child, db); err != nil {
			return err
		}
	}

	return nil
}

func (db *DagBuilderHelper) GetNextDataNode() (*UnixfsNode, error) {
	data := db.Next()
	if data == nil { // we're done!
		return nil, nil
	}

	if len(data) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded
	}

	if db.rawLeaves {
		return &UnixfsNode{
			rawnode: dag.NewRawNode(data),
			raw:     true,
		}, nil
	} else {
		blk := NewUnixfsBlock()
		blk.SetData(data)
		return blk, nil
	}
}

func (db *DagBuilderHelper) Add(node *UnixfsNode) (node.Node, error) {
	dn, err := node.GetDagNode()
	if err != nil {
		return nil, err
	}

	_, err = db.dserv.Add(dn)
	if err != nil {
		return nil, err
	}

	return dn, nil
}

func (db *DagBuilderHelper) Maxlinks() int {
	return db.maxlinks
}

func (db *DagBuilderHelper) Close() error {
	return db.batch.Commit()
}
