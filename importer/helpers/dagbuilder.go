package helpers

import (
	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	"io"
	"os"
)

// DagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type DagBuilderHelper struct {
	dserv    dag.DAGService
	spl      chunk.Splitter
	recvdErr error
	nextData []byte // the next item to return.
	maxlinks int
	batch    *dag.Batch
	fullPath string
	stat     os.FileInfo
}

type DagBuilderParams struct {
	// Maximum number of links per intermediate node
	Maxlinks int

	// DAGService to write blocks to (required)
	Dagserv dag.DAGService
}

// Generate a new DagBuilderHelper from the given params, which data source comes
// from chunks object
func (dbp *DagBuilderParams) New(spl chunk.Splitter) *DagBuilderHelper {
	db := &DagBuilderHelper{
		dserv:    dbp.Dagserv,
		spl:      spl,
		maxlinks: dbp.Maxlinks,
		batch:    dbp.Dagserv.Batch(),
	}
	if fi, ok := spl.Reader().(files.FileInfo); ok {
		db.fullPath = fi.FullPath()
		db.stat = fi.Stat()
	}
	return db
}

// prepareNext consumes the next item from the splitter and puts it
// in the nextData field. it is idempotent-- if nextData is full
// it will do nothing.
func (db *DagBuilderHelper) prepareNext() {
	// if we already have data waiting to be consumed, we're ready
	if db.nextData != nil || db.recvdErr != nil {
		return
	}

	db.nextData, db.recvdErr = db.spl.NextBytes()
	if db.recvdErr == io.EOF {
		db.recvdErr = nil
	}
}

// Done returns whether or not we're done consuming the incoming data.
func (db *DagBuilderHelper) Done() bool {
	// ensure we have an accurate perspective on data
	// as `done` this may be called before `next`.
	db.prepareNext() // idempotent
	if db.recvdErr != nil {
		return false
	}
	return db.nextData == nil
}

// Next returns the next chunk of data to be inserted into the dag
// if it returns nil, that signifies that the stream is at an end, and
// that the current building operation should finish
func (db *DagBuilderHelper) Next() ([]byte, error) {
	db.prepareNext() // idempotent
	d := db.nextData
	db.nextData = nil // signal we've consumed it
	if db.recvdErr != nil {
		return nil, db.recvdErr
	} else {
		return d, nil
	}
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
	data, err := db.Next()
	if err != nil {
		return err
	}
	if data == nil { // we're done!
		return nil
	}

	if len(data) > BlockSizeLimit {
		return ErrSizeLimitExceeded
	}

	node.SetData(data)

	return nil
}

func (db *DagBuilderHelper) SetPosInfo(node *UnixfsNode, offset uint64) {
	if db.stat != nil {
		node.SetPosInfo(offset, db.fullPath, db.stat)
	}
}

func (db *DagBuilderHelper) Add(node *UnixfsNode) (*dag.Node, error) {
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
