package fusemount

import (
	"context"
	"fmt"
	"os"

	"github.com/billziss-gh/cgofuse/fuse"

	config "gx/ipfs/QmUAuYuiafnJRZxDDX7MuruMNsicYNuyub5vUeAcupUBNs/go-ipfs-config"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
	uio "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs/io"
)

/* TODO
func (*FileSystemBase) Getxattr(path string, name string) (int, []byte)
func (*FileSystemBase) Listxattr(path string, fill func(name string) bool) int
*/

//TODO: implement for real
//NOTE: systems manual: "If pathname is a symbolic link, it is dereferenced."
func (fs *FUSEIPFS) Access(path string, mask uint32) int {
	log.Debugf("Access - Request %q{%d}", path, mask)
	return fuseSuccess
	//return -fuse.EACCES
}

func (fs *FUSEIPFS) Statfs(path string, stat *fuse.Statfs_t) int {
	log.Debugf("Statfs - Request %q", path)
	target, err := config.DataStorePath("") //TODO: review
	if err != nil {
		log.Errorf("Statfs - Config err %q: %v", path, err)
		return -fuse.ENOENT //TODO: proper error
	}

	err = fs.fuseFreeSize(stat, target)
	if err != nil {
		log.Errorf("Statfs - Size err %q: %v", target, err)
		return -fuse.ENOENT //TODO: proper error
	}
	return fuseSuccess
}

//FIXME: we need to initialize children if target is a directory
// ^only on readdirplus though
func (fs *FUSEIPFS) Getattr(path string, fStat *fuse.Stat_t, fh uint64) int {
	fs.RLock()
	log.Debugf("Getattr - Request [%X]%q", fh, path)

	/* TODO: we need a way to distinguish file and directory handles
	if fh != invalidIndex {
		curNode, err = fs.LookupHandle(fh)
	} else {
		curNode, err = fs.LookupPath(path)
	}
	*/

	fsNode, err := fs.LookupPath(path)
	if err != nil {
		if err == os.ErrNotExist {
			if !platformException(path) {
				log.Warningf("Getattr - %q not found", path)
			}
			fs.RUnlock()
			return -fuse.ENOENT
		}
		log.Errorf("Getattr - Lookup error %q: %s", path, err)
		fs.RUnlock()
		return -fuse.EIO
	}
	fsNode.Lock()
	fs.RUnlock()
	defer fsNode.Unlock()

	nodeStat, err := fsNode.Stat()
	if err != nil {
		if err == os.ErrNotExist {
			return -fuse.ENOENT
		}
		log.Errorf("Getattr - %q stat err", fsNode, err)
		return -fuse.EIO
	}
	*fStat = *nodeStat
	fStat.Uid, fStat.Gid, _ = fuse.Getcontext()
	return fuseSuccess
}

func canAsync(fsNd fusePath) bool {
	switch fsNd.(type) {
	case *ipfsNode, *ipnsNode, *ipnsKey, *mfsNode, *mfsRoot:
		return true
	}
	return false
}

func mfsSubNodes(ctx context.Context, mRoot *mfs.Root, path string) (<-chan directoryStringEntry, int, error) {

	//NOTE: [2019.03.26] MFS's ForEachEntry is not async, as such we sidestep it and use unixfs directly
	// if ForEachEntry becomes async we do not need the dag service directly
	di := ctx.Value(dagKey{})
	if di == nil {
		return nil, 0, fmt.Errorf("context does not contain dag in value")
	}
	dag, ok := di.(*coreiface.APIDagService)
	if !ok {
		return nil, 0, fmt.Errorf("context value is not a valid dag service")
	}
	//

	mfsNd, err := mfs.Lookup(mRoot, path)
	if err != nil {
		return nil, 0, err
	}

	//mfsDir, ok := mfsNd.(*mfs.Directory)
	//mfsDir.ForEachEntry(tctx, process(entry))
	_, ok = mfsNd.(*mfs.Directory)
	if !ok {
		return nil, 0, fmt.Errorf("mfs %q not a directory", path)
	}

	ipldNd, err := mfsNd.GetNode()
	if err != nil {
		return nil, 0, err
	}

	iStat, err := ipldNd.Stat()
	if err != nil {
		return nil, 0, err
	}
	entries := iStat.NumLinks

	unixDir, err := uio.NewDirectoryFromNode(dag, ipldNd)
	if err != nil {
		return nil, 0, err
	}

	return coreMux(unixDir.EnumLinksAsync(ctx)), entries, nil
}

func ipldStat(fStat *fuse.Stat_t, node ipld.Node) error {
	ufsNode, err := unixfs.ExtractFSNode(ipldNode)
	if err != nil {
		return err
	}

	switch t := ufsNode.Type(); t {
	case unixfs.TFile:
		fStat.Mode |= fuse.S_IFREG
		fStat.Size = int64(ufsNode.FileSize())
	case unixfs.TDirectory, unixfs.THAMTShard:
		fStat.Mode |= fuse.S_IFDIR
		nodeStat, err := node.Stat()
		if err != nil {
			return err
		}
		fStat.Size = nodeStat.NumLinks // NOTE: we're using this as the child count; off_t is not defined for directories in standard
	case unixfs.TSymlink:
		fStat.Mode |= fuse.S_IFLNK
		fStat.Size = len(string(ufsNode.Data()))
	default:
		return fmt.Errorf("unexpected node type %d", t)
	}

	if bs := ufsNode.BlockSizes(); len(bs) != 0 {
		fStat.Blksize = int64(bs[0]) //NOTE: this value is to be used as a hint only; subsequent child block size may differ
	}
	return nil
}

func mfsStat(fStat *fuse.Stat_t, mroot *mfs.Root, path string) error {
	mfsNode, err := mfs.Lookup(mroot, path)
	if err != nil {
		return err
	}

	ipldNode, err := mfsNode.GetNode()
	if err != nil {
		return err
	}

	if err = ipldStat(fStat, ipldNode); err != nil {
		return err
	}

	fStat.Mode |= fuse.S_IWUSR

	return nil
}
