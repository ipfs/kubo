package fsnodes

import (
	"context"
	"fmt"

	"github.com/hugelgupf/p9/p9"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

var _ p9.File = (*RootIndex)(nil)

//TODO: this shouldn't be necessary, consider how best to path the virtual roots and maintain compat with IPFS core
func newRootPath() corepath.Resolved {
	return rootNode("/")
}

type rootNode string

func (rn rootNode) String() string { return string(rn) }
func (rootNode) Namespace() string { return "virtual" }
func (rootNode) Mutable() bool     { return true }
func (rootNode) IsValid() error    { return nil }
func (rootNode) Cid() cid.Cid      { return cid.Cid{} }
func (rootNode) Root() cid.Cid     { return cid.Cid{} }
func (rootNode) Remainder() string { return "" }

//

type RootIndex struct {
	IPFSBase
}

func NewRoot(ctx context.Context, core coreiface.CoreAPI, logger logging.EventLogger) (*RootIndex, error) {
	ri := &RootIndex{
		IPFSBase: IPFSBase{
			Base: Base{
				Ctx:    ctx,
				Logger: logger,
				Qid: p9.QID{
					Type:    p9.TypeDir,
					Version: 1,
					Path:    uint64(pVirtualRoot)}},
			Path: newRootPath(),
			core: core,
		},
	}

	return ri, nil
}

func (ri *RootIndex) Attach() (p9.File, error) {
	ri.Logger.Debugf("RI Attach")
	return ri, nil
}

func (ri *RootIndex) GetAttr(req p9.AttrMask) (p9.QID, p9.AttrMask, p9.Attr, error) {
	ri.Logger.Debugf("RI GetAttr")
	qid := p9.QID{
		Type:    p9.TypeDir,
		Version: 1,
		Path:    uint64(pVirtualRoot),
	}

	//ri.Logger.Errorf("RI mask: %v", req)

	//TODO: [metadata] quick hack; revise
	attr := p9.Attr{
		Mode: p9.ModeDirectory,
		RDev: dMemory,
	}

	attrMask := p9.AttrMask{
		Mode: true,
		RDev: true,
	}

	return qid, attrMask, attr, nil
}
func (ri *RootIndex) Walk(names []string) ([]p9.QID, p9.File, error) {
	ri.Logger.Debugf("RI Walk names %v", names)
	ri.Logger.Debugf("RI Walk myself: %v", ri.Qid)

	if doClone(names) {
		ri.Logger.Debugf("RI Walk cloned")
		return []p9.QID{ri.Qid}, ri, nil
	}

	//NOTE: if doClone is false, it implies len(names) > 0

	var tailFile p9.File
	var subQids []p9.QID
	qids := make([]p9.QID, 0, len(names))

	switch names[0] {
	case "ipfs":
		pinDir, err := initPinFS(ri.Ctx, ri.core, ri.Logger).Attach()
		if err != nil {
			return nil, nil, err
		}
		if subQids, tailFile, err = pinDir.Walk(names[1:]); err != nil {
			return nil, nil, err
		}
		qids = append(qids, subQids...)
	default:
		return nil, nil, fmt.Errorf("%q is not provided by us", names[0]) //TODO: Err vars
	}
	ri.Logger.Debugf("RI Walk reg ret %v, %v", qids, tailFile)
	return qids, tailFile, nil
}

// TODO: check specs for directory iounit size,
// if it's undefined we should repurpose it to return the count to the client
func (ri *RootIndex) Open(mode p9.OpenFlags) (p9.QID, uint32, error) {
	ri.Logger.Debugf("RI Open")
	return ri.Qid, 0, nil
}

func (ri *RootIndex) Readdir(offset uint64, count uint32) ([]p9.Dirent, error) {
	ri.Logger.Debugf("RI Readdir")

	subsystems := [...]indexPath{pIPFSRoot}

	ents := make([]p9.Dirent, 0, len(subsystems))
	for i, subsystem := range subsystems {
		version := 1 //TODO: generate dynamically

		// TODO: allocate root ents array elsewhere
		// modify dynamic fields only here

		ents = append(ents, p9.Dirent{
			Name:   subsystem.String(),
			Offset: uint64(i),
			Type:   p9.TypeDir, //TODO: resolve dynamically
			QID: p9.QID{
				Type:    p9.TypeDir,
				Version: uint32(version), //TODO: maintain version
				Path:    uint64(subsystem),
			},
		})
	}

	//TODO: [all instances; double check] the underlying API may already have do this after we return the slice to it
	if uint32(len(ents)) > count {
		ents = ents[:count]
	}

	ri.Logger.Debugf("RI Readdir returning [%d]ents:%v", len(ents), ents)
	return ents, nil
}
