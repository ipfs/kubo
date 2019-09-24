package fsnodes

import (
	"context"
	"sort"
	"syscall"

	"github.com/djdv/p9/p9"
	cid "github.com/ipfs/go-cid"
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
	file   p9.File
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
	Base
	//subsystems []p9.Dirent
	subsystems map[string]systemTuple
	core       coreiface.CoreAPI
}

// RootAttacher constructs the default RootIndex file system, providing a means to Attach() to it
func RootAttacher(ctx context.Context, core coreiface.CoreAPI, parent p9.File) p9.Attacher {
	ri := &RootIndex{
		core: core,
		Base: Base{
			Logger:    logging.Logger("RootFS"),
			parentCtx: ctx,
			Qid: p9.QID{
				Type: p9.TypeDir,
				Path: cidToQPath(rootPath("/").Cid()),
			},
		},
	}
	ri.meta, ri.metaMask = defaultRootAttr()

	//
	rootDirent := p9.Dirent{
		Type: p9.TypeDir,
		QID:  p9.QID{Type: p9.TypeDir},
	}

	type subattacher func(context.Context, coreiface.CoreAPI, walkRef) p9.Attacher
	type attachTuple struct {
		string
		subattacher
		//p9.Dirent
	}
	subsystems := [...]attachTuple{
		{"ipfs", PinFSAttacher},
		//{"ipns", KeyFsAttacher},
		//{"files", needs CoreAPI changes},
	}

	ri.subsystems = make(map[string]systemTuple, len(subsystems))

	for i, subsystem := range subsystems {
		fs, err := subsystem.subattacher(ctx, core, ri).Attach()
		if err != nil {
			panic(err) // hard implementation error
		}

		//for i, pathUnion := range [...]struct {
		rootDirent.Offset = uint64(i + 1)
		rootDirent.Name = subsystem.string

		rootDirent.QID.Path = cidToQPath(rootPath("/" + subsystem.string).Cid())

		ri.subsystems[subsystem.string] = systemTuple{
			file:   fs,
			dirent: rootDirent,
		}
	}

	if parent != nil {
		ri.parent = parent
	} else {
		ri.parent = ri
	}

	return ri
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("Attach")
	_, err := ri.Base.Attach()
	if err != nil {
		return nil, err
	}

	if ri.parent == ri {
		ri.root = true
	}

	return ri, nil
}

func (ri *RootIndex) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	ri.Logger.Debugf("GetAttr")
	//ri.Logger.Debugf("mask: %v", req)

	return ri.Qid, ri.metaMask, ri.meta, nil
}

func (ri *RootIndex) Walk(names []string) ([]p9.QID, p9.File, error) {
	ri.Logger.Debugf("Walk names %v", names)
	ri.Logger.Debugf("Walk myself: %v", ri.Qid)

	qids := []p9.QID{ri.Qid}

	if ri.open {
		return qids, nil, errWalkOpened
	}

	newFid := new(RootIndex)
	*newFid = *ri
	newFid.root = false

	if shouldClone(names, ri.root) {
		ri.Logger.Debugf("Walk cloned")
		return qids, newFid, nil
	}

	subSys, ok := ri.subsystems[names[0]]
	if !ok {
		ri.Logger.Errorf("%q is not provided by us", names[0])
		return nil, nil, syscall.ENOENT //TODO: migrate to platform independent value
	}

	newFid.child = subSys.file

	return stepper(newFid, names[1:])
}

func (ri *RootIndex) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	ri.Logger.Debugf("Open")
	ri.open = true
	return ri.Qid, 0, nil
}

func (ri *RootIndex) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	ri.Logger.Debugf("Readdir {%d}", count)

	subs := len(ri.subsystems)

	shouldExit, err := boundCheck(offset, subs)
	if shouldExit {
		return nil, err
	}

	// TODO: [perf] iterating over the map and slot populating an array is probably the least wasted effort
	// map => ordered set
	orderedSet := make(systemSlice, 0, subs)
	for _, pair := range ri.subsystems {
		orderedSet = append(orderedSet, pair)
	}
	sort.Sort(orderedSet)

	offsetIndex := subs - int(offset)

	// n-tuple => singleton
	ents := make([]p9.Dirent, 0, offsetIndex)
	for _, pair := range orderedSet[offset:] {
		ents = append(ents, pair.dirent)
	}

	// set => trimmed-set
	if offsetIndex > int(count) {
		return ents[:count], nil
	}

	return ents, nil
}

func (ri *RootIndex) Close() error {
	ri.Logger.Debugf("closing: {%v} root", ri.Qid)
	err := ri.Base.Close()
	ri.open = false
	return err
}
