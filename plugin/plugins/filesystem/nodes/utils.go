package fsnodes

import (
	"context"
	"hash/fnv"
	"time"

	"github.com/djdv/p9/p9"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-unixfs"
	unixpb "github.com/ipfs/go-unixfs/pb"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

func doClone(names []string) bool {
	l := len(names)
	if l < 1 {
		return true
	}
	//TODO: double check the spec to make sure dot handling is correct
	// we may only want to clone on ".." if we're a root
	if pc := names[0]; l == 1 && (pc == ".." || pc == "." || pc == "") {
		return true
	}
	return false
}

//TODO: rename this and/or extend
// it only does some of the stat and not what people probably expect
func coreStat(ctx context.Context, dirEnt *p9.Dirent, core coreiface.CoreAPI, path corepath.Path) error {
	if ipldNode, err := core.ResolveNode(ctx, path); err != nil {
		return err
	} else {
		return ipldStat(dirEnt, ipldNode)
	}
}

func coreGetAttr(ctx context.Context, attr *p9.Attr, attrMask p9.AttrMask, core coreiface.CoreAPI, path corepath.Path) (err error) {
	ipldNode, err := core.ResolveNode(ctx, path)
	if err != nil {
		return err
	}
	ufsNode, err := unixfs.ExtractFSNode(ipldNode)
	if err != nil {
		return err
	}

	if attrMask.Mode {
	attr.Mode = IRXA //TODO: this should probably be the callers responsability; just document that permissions should be set afterwards or something
	attr.Mode |= unixfsTypeTo9Mode(ufsNode.Type())
	}

	if attrMask.Blocks{
	if bs := ufsNode.BlockSizes(); len(bs) != 0 {
		attr.BlockSize = bs[0] //NOTE: this value is to be used as a hint only; subsequent child block size may differ
	}

	//TODO [eventualy]: switch off here for handling of time metadata in new format standard
	timeStamp(attr, attrMask)
}

if attrMask.Size {
	attr.Size, attrMask.Size = ufsNode.FileSize(), true
}

if attrMask.RDev {
	switch path.Namespace() {
	case "ipfs":
		attr.RDev, attrMask.RDev = dIPFS, true
		//case "ipns":
		//attr.RDev, attrMask.RDev = dIPNS, true
		//etc.
	}
}

	return nil
}

func ipldStat(dirEnt *p9.Dirent, node ipld.Node) error {
	ufsNode, err := unixfs.ExtractFSNode(node)

	if err != nil {
		return err
	}

	nodeType := unixfsTypeTo9Mode(ufsNode.Type()).QIDType()

	dirEnt.Type = nodeType
	dirEnt.QID.Type = nodeType
	dirEnt.QID.Path = cidToQPath(node.Cid())

	return nil
}

func cidToQPath(cid cid.Cid) uint64 {
	hasher := fnv.New64a()
	if _, err := hasher.Write(cid.Bytes()); err != nil {
		panic(err)
	}
	return hasher.Sum64()
}

func coreLs(ctx context.Context, corePath corepath.Path, core coreiface.CoreAPI) (<-chan coreiface.DirEntry, error) {

	//FIXME: asyncContext hangs on reset
	//asyncContext := deriveTimerContext(ctx, 10*time.Second)
	asyncContext := ctx

	coreChan, err := core.Unixfs().Ls(asyncContext, corePath)
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

func coreTypeTo9Mode(ct coreiface.FileType) p9.FileMode {
	switch ct {
	// case coreiface.TDirectory, unixfs.THAMTShard // Should we account for this?
	case coreiface.TDirectory:
		return p9.ModeDirectory
	case coreiface.TSymlink:
		return p9.ModeSymlink
	default: //TODO: probably a bad assumption to make
		return p9.ModeRegular
	}
}

//TODO: see if we can remove the need for this; rely only on the core if we can
func unixfsTypeTo9Mode(ut unixpb.Data_DataType) p9.FileMode {
	switch ut {
	// case unixpb.Data_DataDirectory, unixpb.Data_DataHAMTShard // Should we account for this?
	case unixpb.Data_Directory:
		return p9.ModeDirectory
	case unixpb.Data_Symlink:
		return p9.ModeSymlink
	default: //TODO: probably a bad assumption to make
		return p9.ModeRegular
	}
}

func coreEntTo9Ent(coreEnt coreiface.DirEntry) p9.Dirent {
	entType := coreTypeTo9Mode(coreEnt.Type).QIDType()

	return p9.Dirent{
		Name: coreEnt.Name,
		Type: entType,
		QID: p9.QID{
			Type: entType,
			Path: cidToQPath(coreEnt.Cid)}}
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

const ( // pedantic POSIX stuff
	S_IROTH p9.FileMode = p9.Read
	S_IWOTH             = p9.Write
	S_IXOTH             = p9.Exec

	S_IRGRP = S_IROTH << 3
	S_IWGRP = S_IWOTH << 3
	S_IXGRP = S_IXOTH << 3

	S_IRUSR = S_IRGRP << 3
	S_IWUSR = S_IWGRP << 3
	S_IXUSR = S_IXGRP << 3

	S_IRWXO = S_IROTH | S_IWOTH | S_IXOTH
	S_IRWXG = S_IRGRP | S_IWGRP | S_IXGRP
	S_IRWXU = S_IRUSR | S_IWUSR | S_IXUSR

	IRWXA = S_IRWXU | S_IRWXG | S_IRWXO            // 0777
	IRXA  = IRWXA &^ (S_IWUSR | S_IWGRP | S_IWOTH) // 0555
)

func defaultRootAttr() (attr p9.Attr, attrMask p9.AttrMask) {
	attr.Mode = p9.ModeDirectory | IRXA
	attr.RDev = dMemory
	attrMask.Mode = true
	attrMask.RDev = true
	attrMask.Size = true
	//timeStamp(&attr, attrMask)
	return attr, attrMask
}

func timeStamp(attr *p9.Attr, mask p9.AttrMask) {
	now := time.Now()
if mask.ATime {
attr.ATimeSeconds  ,	attr.ATimeNanoSeconds =  uint64(now.Unix()), uint64(now.UnixNano())
	}
	if mask.MTime {
attr.MTimeSeconds  ,	attr.MTimeNanoSeconds =  uint64(now.Unix()), uint64(now.UnixNano())
	}
	if mask.CTime {
attr.CTimeSeconds  ,	attr.CTimeNanoSeconds =  uint64(now.Unix()), uint64(now.UnixNano())
	}
		}

//TODO [name]: "new" implies pointer type; this is for embedded consturction
func newIPFSBase(ctx context.Context, path corepath.Resolved, kind p9.QIDType, core coreiface.CoreAPI, logger logging.EventLogger) IPFSBase {
	return IPFSBase{
		Path: path,
		core: core,
		Base: Base{
			Logger: logger,
			Ctx:    ctx,
			Qid: p9.QID{
				Type: kind,
				Path: cidToQPath(path.Cid())}}}
}
