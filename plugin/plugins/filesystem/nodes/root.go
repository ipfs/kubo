package fsnodes

import (
	"context"
	"fmt"

	"github.com/djdv/p9/p9"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/multiformats/go-multihash"
)

const nRoot = "" // root namespace is intentionally left blank
var _ p9.File = (*RootIndex)(nil)

type rootPath string

func (rp rootPath) String() string { return string(rp) }
func (rootPath) Namespace() string { return nRoot }
func (rp rootPath) Mutable() bool  { return true }
func (rp rootPath) IsValid() error { return nil }
func (rootPath) Root() cid.Cid     { return cid.Cid{} } //TODO: reference the root CID when overlay is finished
func (rootPath) Remainder() string { return "" }
func (rp rootPath) Cid() cid.Cid {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: multihash.BLAKE2B_MIN}
	c, err := prefix.Sum([]byte(rp))
	if err != nil {
		panic(err) //invalid root
	}
	return c
}

// RootIndex is a virtual directory file system, that maps a set of file system implementations to a hierarchy
// Currently: "/":RootIndex, "/ipfs":PinFS, "/ipfs/*:IPFS
type RootIndex struct {
	Base

	subsystems []p9.Dirent

	core coreiface.CoreAPI
}

// RootAttacher constructs the default RootIndex file system, providing a means to Attach() to it
func RootAttacher(ctx context.Context, core coreiface.CoreAPI) *RootIndex {
	ri := &RootIndex{
		core:       core,
		subsystems: make([]p9.Dirent, 0, 1), //NOTE: capacity should be tied to root child count
		Base: Base{
			Logger: logging.Logger("RootFS"),
			Ctx:    ctx,
			Qid: p9.QID{
				Type: p9.TypeDir,
				Path: cidToQPath(rootPath("/").Cid()),
			},
		},
	}
	ri.meta, ri.metaMask = defaultRootAttr()

	rootDirTemplate := p9.Dirent{
		Type: p9.TypeDir,
		QID:  p9.QID{Type: p9.TypeDir}}

	for i, pathUnion := range [...]struct {
		string
		p9.Dirent
	}{
		{"ipfs", rootDirTemplate},
		{"ipns", rootDirTemplate},
	} {
		pathUnion.Dirent.Offset = uint64(i + 1)
		pathUnion.Dirent.Name = pathUnion.string

		pathNode := rootPath("/" + pathUnion.string)
		pathUnion.Dirent.QID.Path = cidToQPath(pathNode.Cid())
		ri.subsystems = append(ri.subsystems, pathUnion.Dirent)
	}

	return ri
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("RI Attach")

	ri.parent = ri
	return ri, nil
}

func (ri *RootIndex) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	ri.Logger.Debugf("RI GetAttr")
	ri.Logger.Debugf("RI mask: %v", req)

	return ri.Qid, ri.metaMask, ri.meta, nil
}

func (ri *RootIndex) Walk(names []string) ([]p9.QID, p9.File, error) {
	ri.Logger.Debugf("RI Walk names %v", names)
	ri.Logger.Debugf("RI Walk myself: %v", ri.Qid)

	if shouldClone(names) {
		ri.Logger.Debugf("RI Walk cloned")
		return []p9.QID{ri.Qid}, ri, nil
	}

	var (
		subSystem walkRef
		err       error
	)

	switch names[0] {
	case "ipfs":
		subSystem = PinFSAttacher(ri.Ctx, ri.core)
	case "ipns":
		subSystem = KeyFSAttacher(ri.Ctx, ri.core)
	default:
		return nil, nil, fmt.Errorf("%q is not provided by us", names[0])
	}

	attacher, ok := subSystem.(p9.Attacher)
	if !ok {
		return nil, nil, fmt.Errorf("%q is not a valid file system", names[0])
	}

	if _, err = attacher.Attach(); err != nil {
		return nil, nil, err
	}

	ri.child = subSystem

	return walker(subSystem, names[1:])
}

func (ri *RootIndex) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	ri.Logger.Debugf("RI Open")
	return ri.Qid, 0, nil
}

func (ri *RootIndex) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	ri.Logger.Debugf("RI Readdir {%d}", count)

	shouldExit, err := boundCheck(offset, len(ri.subsystems))
	if shouldExit {
		return nil, err
	}

	offsetIndex := ri.subsystems[offset:]
	if len(offsetIndex) > int(count) {
		ri.Logger.Debugf("RI Readdir returning [%d]%v\n", count, offsetIndex[:count])
		return offsetIndex[:count], nil
	}

	ri.Logger.Debugf("RI Readdir returning [%d]%v\n", len(offsetIndex), offsetIndex)
	return offsetIndex, nil
}
