package fsnodes

import (
	"context"
	"syscall"

	"github.com/djdv/p9/p9"
	cid "github.com/ipfs/go-cid"
	nodeopts "github.com/ipfs/go-ipfs/plugin/plugins/filesystem/nodes/options"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/multiformats/go-multihash"
)

var _ p9.File = (*RootIndex)(nil)
var _ walkRef = (*RootIndex)(nil)

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
	file   walkRef
	dirent p9.Dirent
}

type systemSlice []systemTuple

func (ss systemSlice) Len() int           { return len(ss) }
func (ss systemSlice) Swap(i, j int)      { ss[i], ss[j] = ss[j], ss[i] }
func (ss systemSlice) Less(i, j int) bool { return ss[i].dirent.Offset < ss[j].dirent.Offset }

//TODO: rename, while this is likely to be the root, it doesn't have to be; maybe "IPFSOverlay"
// RootIndex is a virtual directory file system, that maps a set of file system implementations to a hierarchy
// Currently: "/ipfs":PinFS, "/ipfs/*:IPFS
type RootIndex struct {
	IPFSBase
	subsystems map[string]systemTuple
}

// OverlayFileMeta holds data relevant to file system nodes themselves
type OverlayFileMeta struct {
	// parent may be used to send ".." requests to another file system
	// during `Backtrack`
	parent walkRef
	// proxy may be used to send requests to another file system
	// during `Step`
	proxy walkRef
}

// RootAttacher constructs the default RootIndex file system, providing a means to Attach() to it
func RootAttacher(ctx context.Context, core coreiface.CoreAPI, ops ...nodeopts.AttachOption) p9.Attacher {
	// construct root node
	ri := &RootIndex{IPFSBase: newIPFSBase(ctx, "/", core, ops...)}
	ri.Qid.Type = p9.TypeDir
	ri.meta.Mode, ri.metaMask.Mode = p9.ModeDirectory|IRXA, true

	// attach to subsystems
	// used for proxying walk requests to other filesystems
	type subattacher func(context.Context, coreiface.CoreAPI, ...nodeopts.AttachOption) p9.Attacher
	type attachTuple struct {
		string
		subattacher
		logging.EventLogger
	}

	// "mount/bind" table:
	// "path"=>filesystem
	subsystems := [...]attachTuple{
		{"ipfs", PinFSAttacher, logging.Logger("PinFS")},
		{"ipns", KeyFSAttacher, logging.Logger("KeyFS")},
	}

	// prealloc what we can
	ri.subsystems = make(map[string]systemTuple, len(subsystems))
	opts := []nodeopts.AttachOption{nodeopts.Parent(ri)}
	rootDirent := p9.Dirent{ //template
		Type: p9.TypeDir,
		QID:  p9.QID{Type: p9.TypeDir},
	}

	// couple the strings to their implementations
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

		// add it as a unionlike thing
		ri.subsystems[subsystem.string] = systemTuple{
			file:   fs.(walkRef),
			dirent: rootDirent,
		}
	}

	return ri
}

func (ri *RootIndex) Derive() walkRef {
	newFid := &RootIndex{
		IPFSBase:   ri.IPFSBase.Derive(),
		subsystems: ri.subsystems,
	}

	return newFid
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("Attach")
	return ri, nil
}

func (ri *RootIndex) Walk(names []string) ([]p9.QID, p9.File, error) {
	ri.Logger.Debugf("Walk names %v", names)
	ri.Logger.Debugf("Walk myself: %v", ri.Qid.Path)

	return walker(ri, names)
}

// The RootIndex checks if it has attached to "name"
// derives a node from it, and returns it
func (ri *RootIndex) Step(name string) (walkRef, error) {
	// consume fs/access name
	subSys, ok := ri.subsystems[name]
	if !ok {
		ri.Logger.Errorf("%q is not provided by us", name)
		return nil, syscall.ENOENT //TODO: migrate to platform independent value
	}

	// return a ready to use derivative of it
	return subSys.file.Derive(), nil
}

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

func (ri *RootIndex) Backtrack() (walkRef, error) {
	// return our parent, or ourselves if we don't have one
	if ri.parent != nil {
		return ri.parent, nil
	}
	return ri, nil
}
