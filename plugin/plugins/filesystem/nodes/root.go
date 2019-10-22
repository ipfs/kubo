package fsnodes

import (
	"context"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
	cid "github.com/ipfs/go-cid"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	fsutils "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/utils"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/multiformats/go-multihash"
)

var _ p9.File = (*RootIndex)(nil)
var _ fsutils.WalkRef = (*RootIndex)(nil)

const nRoot = "" // root namespace is intentionally left blank

type rootPath string

func (rp rootPath) String() string { return string(rp) }
func (rootPath) Namespace() string { return nRoot }
func (rootPath) Mutable() bool     { return true }
func (rootPath) IsValid() error    { return nil }
func (rootPath) Root() cid.Cid     { return cid.Cid{} }
func (rootPath) Remainder() string { return "" }
func (rp rootPath) Cid() cid.Cid {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: multihash.BLAKE2B_MIN}
	c, err := prefix.Sum([]byte(rp))
	if err != nil {
		panic(err) //invalid root
	}
	return c
}

type systemTuple struct {
	file   fsutils.WalkRef
	dirent p9.Dirent
}

//TODO: rename, while this is likely to be the root, it doesn't have to be; maybe "IPFSOverlay"
// RootIndex is a virtual directory file system, that maps a set of file system implementations to a hierarchy
// Currently: "/ipfs":PinFS, "/ipfs/*:IPFS
type RootIndex struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr

	IPFSBase
	subsystems map[string]systemTuple
}

// OverlayFileMeta holds data relevant to file system nodes themselves
type OverlayFileMeta struct {
	// parent may be used to send ".." requests to another file system
	// during `Backtrack`
	parent fsutils.WalkRef
	// proxy may be used to send requests to another file system
	// during `Step`
	proxy fsutils.WalkRef
}

// RootAttacher constructs the default RootIndex file system, providing a means to Attach() to it
func RootAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	// construct root node
	ri := &RootIndex{IPFSBase: newIPFSBase(ctx, "/", core, ops...)}
	ri.qid.Type = p9.TypeDir
	ri.meta.Mode, ri.metaMask.Mode = p9.ModeDirectory|IRXA, true

	// attach to subsystems
	// used for proxying walk requests to other filesystems
	type subattacher func(context.Context, coreiface.CoreAPI, ...nodeopts.AttachOption) p9.Attacher
	type attachTuple struct {
		string
		subattacher
		logging.EventLogger
	}

	// 9P Access names mapped to IPFS attacher functions
	subsystems := [...]attachTuple{
		{"ipfs", PinFSAttacher, logging.Logger("PinFS")},
		{"ipns", KeyFSAttacher, logging.Logger("KeyFS")},
	}

	// allocate root entry pairs
	// assign inherent options,
	// and instantiate a template root entry
	ri.subsystems = make(map[string]systemTuple, len(subsystems))
	opts := []nodeopts.AttachOption{nodeopts.Parent(ri)}
	rootDirent := p9.Dirent{
		Type: p9.TypeDir,
		QID:  p9.QID{Type: p9.TypeDir},
	}

	// couple the strings to their implementations
	// "aname"=>{filesystem,entry}
	for i, subsystem := range subsystems {
		logOpt := nodeopts.Logger(subsystem.EventLogger)
		// the file system implementation
		fs, err := subsystem.subattacher(ctx, core, append(opts, logOpt)...).Attach()
		if err != nil {
			panic(err) // hard implementation error
		}

		// create a directory entry for it
		rootDirent.Offset = uint64(i + 1)
		rootDirent.Name = subsystem.string

		rootDirent.QID.Path = cidToQPath(rootPath("/" + subsystem.string).Cid())

		// add the fs+entry to the list of subsystems
		ri.subsystems[subsystem.string] = systemTuple{
			file:   fs.(fsutils.WalkRef),
			dirent: rootDirent,
		}
	}

	return ri
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("Attach")

	newFid := &RootIndex{
		IPFSBase:   ri.IPFSBase.clone(), // root has no paths to walk; don't set node up for change
		subsystems: ri.subsystems,
	}

	// set new fs context
	if err := newFid.forkFilesystem(); err != nil {
		return nil, err
	}
	return newFid, nil
}

func (ri *RootIndex) Open(mode p9.OpenFlags) (p9.QID, uint32, error) { return *ri.qid, 0, nil }
func (ri *RootIndex) Close() error                                   { return ri.IPFSBase.close() }

func (ri *RootIndex) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	ri.Logger.Debugf("Readdir {%d}", count)

	subs := len(ri.subsystems)

	shouldExit, err := boundCheck(offset, subs)
	if shouldExit {
		return nil, err
	}

	relativeEnd := subs - int(offset)

	// use the lesser for allocating the slice
	var ents []p9.Dirent
	if count < uint32(relativeEnd) {
		ents = make([]p9.Dirent, count)
	} else {
		ents = make([]p9.Dirent, relativeEnd)
	}

	// use ents from map within request bounds to populate slice
	for _, pair := range ri.subsystems {
		if count == 0 {
			break
		}
		if pair.dirent.Offset >= offset && pair.dirent.Offset <= uint64(relativeEnd) {
			ents[pair.dirent.Offset-1] = pair.dirent
			count--
		}
	}

	return ents, nil
}

/* WalkRef relevant */

func (ri *RootIndex) Fork() (fsutils.WalkRef, error) {
	newFid := &RootIndex{
		IPFSBase:   ri.IPFSBase.clone(), // root has no paths to walk; don't set node up for change
		subsystems: ri.subsystems,
	}

	// set new operations context
	err := newFid.forkOperations()
	return newFid, err
}

// The RootIndex checks if it has attached to "name"
// derives a node from it, and returns it
func (ri *RootIndex) Step(name string) (fsutils.WalkRef, error) {
	// consume fs/access name
	subSys, ok := ri.subsystems[name]
	if !ok {
		ri.Logger.Errorf("%q is not provided by us", name)
		return nil, ENOENT
	}

	// return a ready to use derivative of it
	return subSys.file.Fork()
}

func (ri *RootIndex) CheckWalk() error                    { return ri.Base.checkWalk() }
func (ri *RootIndex) QID() (p9.QID, error)                { return ri.Base.qID() }
func (ri *RootIndex) Backtrack() (fsutils.WalkRef, error) { return ri.IPFSBase.backtrack(ri) }

/* base class boilerplate */

func (ri *RootIndex) Walk(names []string) ([]p9.QID, p9.File, error) {
	ri.Logger.Debugf("Walk names %v", names)
	ri.Logger.Debugf("Walk myself: %v", ri.qid.Path)

	return fsutils.Walker(ri, names)
}
