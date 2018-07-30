package helpers

import (
	"context"
	"io"
	"os"

	dag "gx/ipfs/QmRy4Qk9hbgFX9NGJRm8rBThrA8PZhNCitMgeRYyZ67s59/go-merkledag"
	ft "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs"
	pb "gx/ipfs/QmSaz8Qg77gGqvDvLKeSAY7ivDEnramSWF6T7TcRwFpHtP/go-unixfs/pb"

	pi "gx/ipfs/QmSHjPDw8yNgLZ7cBfX7w3Smn7PHwYhNEpd4LHQQxUg35L/go-ipfs-posinfo"
	chunker "gx/ipfs/QmVDjhUMtkRskBFAVNwyXuLSKbeAya7JKPnzAxMKDaK4x4/go-ipfs-chunker"
	cid "gx/ipfs/QmYVNvtQkeZ6AKSwDrjQTs432QtL6umrrK41EBq3cu7iSP/go-cid"
	ipld "gx/ipfs/QmZtNq8dArGfnpCZfx2pUNY7UcjGhVp5qqwQ4hH6mpTMRQ/go-ipld-format"
	files "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit/files"
)

// DagBuilderHelper wraps together a bunch of objects needed to
// efficiently create unixfs dag trees
type DagBuilderHelper struct {
	dserv     ipld.DAGService
	spl       chunker.Splitter
	recvdErr  error
	rawLeaves bool
	nextData  []byte // the next item to return.
	maxlinks  int
	batch     *ipld.Batch
	prefix    *cid.Prefix

	// Filestore support variables.
	// ----------------------------
	// TODO: Encapsulate in `FilestoreNode` (which is basically what they are).
	//
	// Besides having the path this variable (if set) is used as a flag
	// to indicate that Filestore should be used.
	fullPath string
	stat     os.FileInfo
	// Keeps track of the current file size added to the DAG (used in
	// the balanced builder). It is assumed that the `DagBuilderHelper`
	// is not reused to construct another DAG, but a new one (with a
	// zero `offset`) is created.
	offset uint64
}

// DagBuilderParams wraps configuration options to create a DagBuilderHelper
// from a chunker.Splitter.
type DagBuilderParams struct {
	// Maximum number of links per intermediate node
	Maxlinks int

	// RawLeaves signifies that the importer should use raw ipld nodes as leaves
	// instead of using the unixfs TRaw type
	RawLeaves bool

	// CID Prefix to use if set
	Prefix *cid.Prefix

	// DAGService to write blocks to (required)
	Dagserv ipld.DAGService

	// NoCopy signals to the chunker that it should track fileinfo for
	// filestore adds
	NoCopy bool

	// URL if non-empty (and NoCopy is also true) indicates that the
	// file will not be stored in the datastore but instead retrieved
	// from this location via the urlstore.
	URL string
}

// New generates a new DagBuilderHelper from the given params and a given
// chunker.Splitter as data source.
func (dbp *DagBuilderParams) New(spl chunker.Splitter) *DagBuilderHelper {
	db := &DagBuilderHelper{
		dserv:     dbp.Dagserv,
		spl:       spl,
		rawLeaves: dbp.RawLeaves,
		prefix:    dbp.Prefix,
		maxlinks:  dbp.Maxlinks,
		batch:     ipld.NewBatch(context.TODO(), dbp.Dagserv),
	}
	if fi, ok := spl.Reader().(files.FileInfo); dbp.NoCopy && ok {
		db.fullPath = fi.AbsPath()
		db.stat = fi.Stat()
	}

	if dbp.URL != "" && dbp.NoCopy {
		db.fullPath = dbp.URL
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
// that the current building operation should finish.
func (db *DagBuilderHelper) Next() ([]byte, error) {
	db.prepareNext() // idempotent
	d := db.nextData
	db.nextData = nil // signal we've consumed it
	if db.recvdErr != nil {
		return nil, db.recvdErr
	}
	return d, nil
}

// GetDagServ returns the dagservice object this Helper is using
func (db *DagBuilderHelper) GetDagServ() ipld.DAGService {
	return db.dserv
}

// NewUnixfsNode creates a new Unixfs node to represent a file.
func (db *DagBuilderHelper) NewUnixfsNode() *UnixfsNode {
	n := &UnixfsNode{
		node: new(dag.ProtoNode),
		ufmt: ft.NewFSNode(ft.TFile),
	}
	n.SetPrefix(db.prefix)
	return n
}

// GetPrefix returns the internal `cid.Prefix` set in the builder.
func (db *DagBuilderHelper) GetPrefix() *cid.Prefix {
	return db.prefix
}

// NewLeaf creates a leaf node filled with data.  If rawLeaves is
// defined than a raw leaf will be returned.  Otherwise, if data is
// nil the type field will be TRaw (for backwards compatibility), if
// data is defined (but possibly empty) the type field will be TRaw.
func (db *DagBuilderHelper) NewLeaf(data []byte) (*UnixfsNode, error) {
	if len(data) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded
	}

	if db.rawLeaves {
		if db.prefix == nil {
			return &UnixfsNode{
				rawnode: dag.NewRawNode(data),
				raw:     true,
			}, nil
		}
		rawnode, err := dag.NewRawNodeWPrefix(data, *db.prefix)
		if err != nil {
			return nil, err
		}
		return &UnixfsNode{
			rawnode: rawnode,
			raw:     true,
		}, nil
	}

	if data == nil {
		return db.NewUnixfsNode(), nil
	}

	blk := db.newUnixfsBlock()
	blk.SetData(data)
	return blk, nil
}

// NewLeafNode is a variation from `NewLeaf` (see its description) that
// returns an `ipld.Node` instead.
func (db *DagBuilderHelper) NewLeafNode(data []byte) (ipld.Node, error) {
	if len(data) > BlockSizeLimit {
		return nil, ErrSizeLimitExceeded
	}

	if db.rawLeaves {
		// Encapsulate the data in a raw node.
		if db.prefix == nil {
			return dag.NewRawNode(data), nil
		}
		rawnode, err := dag.NewRawNodeWPrefix(data, *db.prefix)
		if err != nil {
			return nil, err
		}
		return rawnode, nil
	}

	// Encapsulate the data in UnixFS node (instead of a raw node).
	fsNodeOverDag := db.NewFSNodeOverDag(ft.TFile)
	fsNodeOverDag.SetFileData(data)
	node, err := fsNodeOverDag.Commit()
	if err != nil {
		return nil, err
	}
	// TODO: Encapsulate this sequence of calls into a function that
	// just returns the final `ipld.Node` avoiding going through
	// `FSNodeOverDag`.
	// TODO: Using `TFile` for backwards-compatibility, a bug in the
	// balanced builder was causing the leaf nodes to be generated
	// with this type instead of `TRaw`, the one that should be used
	// (like the trickle builder does).
	// (See https://github.com/ipfs/go-ipfs/pull/5120.)

	return node, nil
}

// newUnixfsBlock creates a new Unixfs node to represent a raw data block
func (db *DagBuilderHelper) newUnixfsBlock() *UnixfsNode {
	n := &UnixfsNode{
		node: new(dag.ProtoNode),
		ufmt: ft.NewFSNode(ft.TRaw),
	}
	n.SetPrefix(db.prefix)
	return n
}

// FillNodeLayer will add datanodes as children to the give node until
// at most db.indirSize nodes are added.
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

// GetNextDataNode builds a UnixFsNode with the data obtained from the
// Splitter, given the constraints (BlockSizeLimit, RawLeaves) specified
// when creating the DagBuilderHelper.
func (db *DagBuilderHelper) GetNextDataNode() (*UnixfsNode, error) {
	data, err := db.Next()
	if err != nil {
		return nil, err
	}

	if data == nil { // we're done!
		return nil, nil
	}

	return db.NewLeaf(data)
}

// NewLeafDataNode is a variation of `GetNextDataNode` that returns
// an `ipld.Node` instead. It builds the `node` with the data obtained
// from the Splitter and returns it with the `dataSize` (that will be
// used to keep track of the DAG file size). The size of the data is
// computed here because after that it will be hidden by `NewLeafNode`
// inside a generic `ipld.Node` representation.
func (db *DagBuilderHelper) NewLeafDataNode() (node ipld.Node, dataSize uint64, err error) {
	fileData, err := db.Next()
	if err != nil {
		return nil, 0, err
	}
	dataSize = uint64(len(fileData))

	// Create a new leaf node containing the file chunk data.
	node, err = db.NewLeafNode(fileData)
	if err != nil {
		return nil, 0, err
	}

	// Convert this leaf to a `FilestoreNode` if needed.
	node = db.ProcessFileStore(node, dataSize)

	return node, dataSize, nil
}

// ProcessFileStore generates, if Filestore is being used, the
// `FilestoreNode` representation of the `ipld.Node` that
// contains the file data. If Filestore is not being used just
// return the same node to continue with its addition to the DAG.
//
// The `db.offset` is updated at this point (instead of when
// `NewLeafDataNode` is called, both work in tandem but the
// offset is more related to this function).
func (db *DagBuilderHelper) ProcessFileStore(node ipld.Node, dataSize uint64) ipld.Node {
	// Check if Filestore is being used.
	if db.fullPath != "" {
		// Check if the node is actually a raw node (needed for
		// Filestore support).
		if _, ok := node.(*dag.RawNode); ok {
			fn := &pi.FilestoreNode{
				Node: node,
				PosInfo: &pi.PosInfo{
					Offset:   db.offset,
					FullPath: db.fullPath,
					Stat:     db.stat,
				},
			}

			// Update `offset` with the size of the data generated by `db.Next`.
			db.offset += dataSize

			return fn
		}
	}

	// Filestore is not used, return the same `node` argument.
	return node
}

// Add sends a node to the DAGService, and returns it.
func (db *DagBuilderHelper) Add(node *UnixfsNode) (ipld.Node, error) {
	dn, err := node.GetDagNode()
	if err != nil {
		return nil, err
	}

	err = db.dserv.Add(context.TODO(), dn)
	if err != nil {
		return nil, err
	}

	return dn, nil
}

// Maxlinks returns the configured maximum number for links
// for nodes built with this helper.
func (db *DagBuilderHelper) Maxlinks() int {
	return db.maxlinks
}

// Close has the DAGService perform a batch Commit operation.
// It should be called at the end of the building process to make
// sure all data is persisted.
func (db *DagBuilderHelper) Close() error {
	return db.batch.Commit()
}

// AddNodeAndClose adds the last `ipld.Node` from the DAG and
// closes the builder. It returns the same `node` passed as
// argument.
func (db *DagBuilderHelper) AddNodeAndClose(node ipld.Node) (ipld.Node, error) {
	err := db.batch.Add(node)
	if err != nil {
		return nil, err
	}

	err = db.Close()
	if err != nil {
		return nil, err
	}

	return node, nil
}

// FSNodeOverDag encapsulates an `unixfs.FSNode` that will be stored in a
// `dag.ProtoNode`. Instead of just having a single `ipld.Node` that
// would need to be constantly (un)packed to access and modify its
// internal `FSNode` in the process of creating a UnixFS DAG, this
// structure stores an `FSNode` cache to manipulate it (add child nodes)
// directly , and only when the node has reached its final (immutable) state
// (signaled by calling `Commit()`) is it committed to a single (indivisible)
// `ipld.Node`.
//
// It is used mainly for internal (non-leaf) nodes, and for some
// representations of data leaf nodes (that don't use raw nodes or
// Filestore).
//
// It aims to replace the `UnixfsNode` structure which encapsulated too
// many possible node state combinations.
//
// TODO: Revisit the name.
type FSNodeOverDag struct {
	dag  *dag.ProtoNode
	file *ft.FSNode
}

// NewFSNodeOverDag creates a new `dag.ProtoNode` and `ft.FSNode`
// decoupled from one onther (and will continue in that way until
// `Commit` is called), with `fsNodeType` specifying the type of
// the UnixFS layer node (either `File` or `Raw`).
func (db *DagBuilderHelper) NewFSNodeOverDag(fsNodeType pb.Data_DataType) *FSNodeOverDag {
	node := new(FSNodeOverDag)
	node.dag = new(dag.ProtoNode)
	node.dag.SetPrefix(db.GetPrefix())

	node.file = ft.NewFSNode(fsNodeType)

	return node
}

// AddChild adds a `child` `ipld.Node` to both node layers. The
// `dag.ProtoNode` creates a link to the child node while the
// `ft.FSNode` stores its file size (that is, not the size of the
// node but the size of the file data that it is storing at the
// UnixFS layer). The child is also stored in the `DAGService`.
func (n *FSNodeOverDag) AddChild(child ipld.Node, fileSize uint64, db *DagBuilderHelper) error {
	err := n.dag.AddNodeLink("", child)
	if err != nil {
		return err
	}

	n.file.AddBlockSize(fileSize)

	return db.batch.Add(child)
}

// Commit unifies (resolves) the cache nodes into a single `ipld.Node`
// that represents them: the `ft.FSNode` is encoded inside the
// `dag.ProtoNode`.
//
// TODO: Evaluate making it read-only after committing.
func (n *FSNodeOverDag) Commit() (ipld.Node, error) {
	fileData, err := n.file.GetBytes()
	if err != nil {
		return nil, err
	}
	n.dag.SetData(fileData)

	return n.dag, nil
}

// NumChildren returns the number of children of the `ft.FSNode`.
func (n *FSNodeOverDag) NumChildren() int {
	return n.file.NumChildren()
}

// FileSize returns the `Filesize` attribute from the underlying
// representation of the `ft.FSNode`.
func (n *FSNodeOverDag) FileSize() uint64 {
	return n.file.FileSize()
}

// SetFileData stores the `fileData` in the `ft.FSNode`. It
// should be used only when `FSNodeOverDag` represents a leaf
// node (internal nodes don't carry data, just file sizes).
func (n *FSNodeOverDag) SetFileData(fileData []byte) {
	n.file.SetData(fileData)
}
