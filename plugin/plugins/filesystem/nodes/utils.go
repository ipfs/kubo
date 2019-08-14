package fsnodes

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/hugelgupf/p9/p9"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-unixfs"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

func doClone(names []string) bool {
	l := len(names)
	if l < 1 {
		return true
	}
	//TODO: double check the spec to make sure dot handling is correct
	if pc := names[0]; l == 1 && (pc == ".." || pc == "." || pc == "") {
		return true
	}
	return false
}

//TODO: rename this and/or extend
// it only does some of the stat and not what people probably expect
func coreStat(ctx context.Context, dirEnt *p9.Dirent, core coreiface.CoreAPI, path corepath.Path) (err error) {
	var ipldNode ipld.Node
	if ipldNode, err = core.ResolveNode(ctx, path); err != nil {
		return
	}
	err = ipldStat(dirEnt, ipldNode)
	return
}

//TODO: consider how we want to use AttrMask
// instead of filling it we can use it to only populate requested fields (as is intended)
func coreGetAttr(ctx context.Context, attr *p9.Attr, attrMask *p9.AttrMask, core coreiface.CoreAPI, path corepath.Path) (err error) {
	ipldNode, err := core.ResolveNode(ctx, path)
	if err != nil {
		return err
	}
	ufsNode, err := unixfs.ExtractFSNode(ipldNode)
	if err != nil {
		return err
	}
	attr.Mode = p9.Read | p9.Exec
	switch t := ufsNode.Type(); t {
	case unixfs.TFile:
		attr.Mode |= p9.ModeRegular
	case unixfs.TDirectory, unixfs.THAMTShard:
		attr.Mode |= p9.ModeDirectory
	case unixfs.TSymlink:
		attr.Mode |= p9.ModeSymlink
	default:
		return fmt.Errorf("unexpected node type %d", t)
	}
	attrMask.Mode = true

	if bs := ufsNode.BlockSizes(); len(bs) != 0 {
		attr.BlockSize = bs[0] //NOTE: this value is to be used as a hint only; subsequent child block size may differ
	}

	attr.Size, attrMask.Size = ufsNode.FileSize(), true

	switch path.Namespace() {
	case "ipfs":
		attr.RDev, attrMask.RDev = dIPFS, true
		//case "ipns":
		//attr.RDev, attrMask.RDev = dIPNS, true
		//etc.
	}

	//TODO: rdev; switch off namespace => dIpfs, dIpns, etc.
	//Blocks
	return nil
}

func ipldStat(dirEnt *p9.Dirent, node ipld.Node) error {
	ufsNode, err := unixfs.ExtractFSNode(node)

	if err != nil {
		return err
	}

	nodeType := coreTypeToQtype(coreiface.FileType(ufsNode.Type())) //eww

	dirEnt.Type = nodeType
	dirEnt.QID.Type = nodeType
	dirEnt.QID.Version = 1
	dirEnt.QID.Path = cidToQPath(node.Cid())

	return nil
}

func cidToQPath(cid cid.Cid) (path uint64) {
	buf := bytes.NewReader(cid.Bytes())
	err := binary.Read(buf, binary.LittleEndian, &path)
	if err != nil {
		panic(err)
	}
	return
}

func coreLs(ctx context.Context, corePath corepath.Path, core coreiface.CoreAPI) (<-chan coreiface.DirEntry, error) {

	//FIXME: asyncContext hangs on reset
	//asyncContext := deriveTimerContext(ctx, 10*time.Second)
	asyncContext := ctx

	coreChan, err := core.Unixfs().Ls(asyncContext, corePath, coreoptions.Unixfs.ResolveChildren(false))
	if err != nil {
		//asyncContext.Cancel()
		return nil, err
	}

	oStat, err := core.Object().Stat(asyncContext, corePath)
	if err != nil {
		return nil, err
	}

	relayChan := make(chan coreiface.DirEntry)
	go func() {
		//defer asyncContext.Cancel()
		defer close(relayChan)

		for i := 0; i != oStat.NumLinks; i++ {
			select {
			case <-asyncContext.Done():
				return
			case msg, ok := <-coreChan:
				if !ok {
					return
				}
				if msg.Err != nil {
					relayChan <- msg
					return
				}
				relayChan <- msg
				//asyncContext.Reset() //reset timeout for each entry we receive successfully
			}
		}
	}()

	return relayChan, err
}

func coreTypeToQtype(ct coreiface.FileType) p9.QIDType {
	switch ct {
	// case coreiface.TDirectory, unixfs.THAMTShard: Should we account for this?
	case coreiface.TDirectory:
		return p9.TypeDir
	case coreiface.TSymlink:
		return p9.TypeSymlink
	default:
		return p9.TypeRegular
	}
}

func coreEntTo9Ent(coreEnt coreiface.DirEntry) p9.Dirent {
	entType := coreTypeToQtype(coreEnt.Type)

	return p9.Dirent{
		Name: coreEnt.Name,
		Type: entType,
		QID: p9.QID{
			Type:    entType,
			Version: 1,
			Path:    cidToQPath(coreEnt.Cid)}}
}

type timerContextActual struct {
	context.Context
	cancel context.CancelFunc
	timer  time.Timer
	grace  time.Duration
}

func (tctx timerContextActual) Reset() {
	if !tctx.timer.Stop() {
		<-tctx.timer.C
	}
	tctx.timer.Reset(tctx.grace)
}

func (tctx timerContextActual) Cancel() {
	tctx.cancel()
	if !tctx.timer.Stop() {
		<-tctx.timer.C
	}
}

type timerContext interface {
	context.Context
	Reset()
	Cancel()
}

func deriveTimerContext(ctx context.Context, grace time.Duration) timerContext {
	asyncContext, cancel := context.WithCancel(ctx)
	timer := time.AfterFunc(grace, cancel)
	tctx := timerContextActual{Context: asyncContext,
		cancel: cancel,
		grace:  grace,
		timer:  *timer}

	return tctx
}
