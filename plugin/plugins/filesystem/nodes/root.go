package fsnodes

import (
	"context"
	"fmt"

	"github.com/djdv/p9/p9"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/multiformats/go-multihash"
)

var _ p9.File = (*RootIndex)(nil)

func newRootPath(path string) corepath.Resolved {
	return rootNode(path)
}

type rootNode string

func (rn rootNode) String() string { return string(rn) }
func (rootNode) Namespace() string { return nRoot }
func (rootNode) Mutable() bool     { return true }
func (rootNode) IsValid() error    { return nil } //TODO: should return ENotImpl
func (rn rootNode) Cid() cid.Cid {
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: multihash.BLAKE2B_MIN}
	c, err := prefix.Sum([]byte(rn))
	if err != nil {
		panic(err) //invalid root
	}
	return c
}
func (rootNode) Root() cid.Cid { //TODO: this should probably reference a package variable set during init `rootCid`
	prefix := cid.V1Builder{Codec: cid.DagCBOR, MhType: multihash.BLAKE2B_MIN}
	c, err := prefix.Sum([]byte("/"))
	if err != nil {
		panic(err) //invalid root
	}
	return c
}
func (rootNode) Remainder() string { return "" }

// RootIndex is a virtual directory file system, that maps a set of filesystem implementations to a hierarchy
// Currently: "/":RootIndex, "/ipfs":PinFS, "/ipfs/*:IPFS
type RootIndex struct {
	IPFSBase
	subsystems []p9.Dirent
}

// RootAttacher constructs the default RootIndex file system, providing a means to Attach() to it
func RootAttacher(ctx context.Context, core coreiface.CoreAPI) *RootIndex {
	ri := &RootIndex{
		IPFSBase: newIPFSBase(ctx, newRootPath("/"), p9.TypeDir,
			core, logging.Logger("RootFS")),
		subsystems: make([]p9.Dirent, 0, 1)} //TODO: [const]: dirent count
	ri.Qid.Path = cidToQPath(ri.Path.Cid())

	ri.meta, ri.metaMask = defaultRootAttr()

	rootDirTemplate := p9.Dirent{
		Type: p9.TypeDir,
		QID:  p9.QID{Type: p9.TypeDir}}

	for i, pathUnion := range [...]struct {
		string
		p9.Dirent
	}{
		{"ipfs", rootDirTemplate},
		//{"ipns", rootDirTemplate},
	} {
		pathUnion.Dirent.Offset = uint64(i + 1)
		pathUnion.Dirent.Name = pathUnion.string

		pathNode := newRootPath("/" + pathUnion.string)
		pathUnion.Dirent.QID.Path = cidToQPath(pathNode.Cid())
		ri.subsystems = append(ri.subsystems, pathUnion.Dirent)
	}

	return ri
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("RI Attach")
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

	if doClone(names) {
		ri.Logger.Debugf("RI Walk cloned")
		return []p9.QID{ri.Qid}, ri, nil
	}

	//NOTE: if doClone is false, it implies len(names) > 0
	switch names[0] {
	case "ipfs":
		pinDir, err := PinFSAttacher(ri.Ctx, ri.core).Attach()
		if err != nil {
			return nil, nil, err
		}
		return pinDir.Walk(names[1:])
	default:
		return nil, nil, fmt.Errorf("%q is not provided by us", names[0]) //TODO: Err vars
	}
}

func (ri *RootIndex) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	ri.Logger.Debugf("RI Open")
	return ri.Qid, 0, nil
}

func (ri *RootIndex) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	ri.Logger.Debugf("RI Readdir {%d}", count)
	sLen := uint64(len(ri.subsystems))

	if offset >= sLen {
		return nil, nil //TODO: [spec] should we error here?
	}

	offsetIndex := ri.subsystems[offset:]
	if len(offsetIndex) > int(count) {
		ri.Logger.Debugf("RI Readdir returning [%d]%v\n", count, offsetIndex[:count])
		return offsetIndex[:count], nil
	}

	ri.Logger.Debugf("RI Readdir returning [%d]%v\n", len(offsetIndex), offsetIndex)
	return offsetIndex, nil
}
