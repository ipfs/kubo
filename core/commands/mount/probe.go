package fusemount

import (
	"fmt"

	"github.com/billziss-gh/cgofuse/fuse"

	config "gx/ipfs/QmUAuYuiafnJRZxDDX7MuruMNsicYNuyub5vUeAcupUBNs/go-ipfs-config"
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
		if !platformException(path) {
			log.Warningf("Getattr - Lookup error %q: %s", path, err)
		}
		fs.RUnlock()
		return -fuse.ENOENT
	}
	fsNode.Lock()
	fs.RUnlock()
	defer fsNode.Unlock()

	//NOTE [2018.12.26]: [uid]
	/* fuse.Getcontext only contains data in callstack under:
	- Mknod
	- Mkdir
	- Getattr
	- Open
	- OpenDir
	- Create
	TODO: we need to retain values from chmod and chown and not overwrite them here
	*/

	nodeStat := fsNode.Stat()
	if nodeStat == nil {
		log.Errorf("Getattr - node %q was not initialized properly", fsNode)
		return -fuse.EIO
	}

	if nodeStat.Mode != 0 { // active node retrieved from lookup
		*fStat = *nodeStat
		fStat.Uid, fStat.Gid, _ = fuse.Getcontext()
		return fuseSuccess
	}

	// Local permissions
	var permissionBits uint32 = 0555
	switch fsNode.(type) {
	case *mfsNode, *mfsRoot, *ipnsRoot, *ipnsKey:
		permissionBits |= 0220
	case *ipnsNode:
		keyName, _, _ := ipnsSplit(path)
		if _, err := resolveKeyName(fs.ctx, fs.core.Key(), keyName); err == nil { // we own this path/key locally
			permissionBits |= 0220
		}
	}
	nodeStat.Mode = permissionBits

	// POSIX type + sizes
	switch fsNode.(type) {
	case *mountRoot, *ipfsRoot, *ipnsRoot, *mfsRoot:
		nodeStat.Mode |= fuse.S_IFDIR
	default:
		globalNode := fsNode
		if isReference(fsNode) {
			globalNode, err = fs.resolveToGlobal(fsNode)
			if err != nil {
				log.Errorf("Getattr - reference node %q could not be resolved: %s ", fsNode, err)
				return -fuse.EIO
			}
		}

		ipldNode, err := fs.core.ResolveNode(fs.ctx, globalNode)
		if err != nil {
			log.Errorf("Getattr - reference node %q could not be resolved: %s ", fsNode, err)
			return -fuse.EIO
		}
		ufsNode, err := unixfs.ExtractFSNode(ipldNode)
		if err != nil {
			log.Errorf("Getattr - reference node %q could not be transformed into UnixFS type: %s ", fsNode, err)
			return -fuse.EIO
		}

		switch ufsNode.Type() {
		case unixfs.TFile:
			nodeStat.Mode |= fuse.S_IFREG
		case unixfs.TDirectory:
			nodeStat.Mode |= fuse.S_IFDIR
		case unixfs.TSymlink:
			nodeStat.Mode |= fuse.S_IFLNK
		default:
			log.Errorf("Getattr - unexpected node type {%T}%q", ufsNode, globalNode)
			return -fuse.EIO
		}

		if bs := ufsNode.BlockSizes(); len(bs) != 0 {
			nodeStat.Blksize = int64(bs[0])
		}
		nodeStat.Size = int64(ufsNode.FileSize())
	}

	// Time
	now := fuse.Now()
	switch fsNode.(type) {
	case *mountRoot, *ipfsRoot, *ipnsRoot, *mfsRoot:
		nodeStat.Birthtim = fs.mountTime
	default:
		nodeStat.Birthtim = now
	}

	nodeStat.Atim, nodeStat.Mtim, nodeStat.Ctim = now, now, now //!!!

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

func (fs *FUSEIPFS) ipnsRootSubnodes() []directoryEntry {
	keys, err := fs.core.Key().List(fs.ctx)
	if err != nil {
		log.Errorf("ipnsRoot - Key err: %v", err)
		return nil
	}

	ents := make([]directoryEntry, 0, len(keys))
	if !fReaddirPlus {
		for _, key := range keys {
			ents = append(ents, directoryEntry{label: key.Name()})
		}
		return ents
	}
	//TODO [readdirplus]
	return nil
}

//TODO: accept context arg
func (fs *FUSEIPFS) mfsSubNodes(filesRoot *mfs.Root, path string) (<-chan unixfs.LinkResult, int, error) {
	//log.Errorf("mfsSubNodes dbg dir %q", path)
	mfsNd, err := mfs.Lookup(filesRoot, path)
	if err != nil {
		return nil, 0, err
	}

	//mfsDir, ok := mfsNd.(*mfs.Directory)
	_, ok := mfsNd.(*mfs.Directory)
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
	// [2019.02.06] MFS's ForEachEntry is not async, so this conflicts with our Readdir timeout timer
	//go mfsDir.ForEachEntry(fs.ctx, muxMessage)

	unixDir, err := uio.NewDirectoryFromNode(fs.core.Dag(), ipldNd)
	if err != nil {
		return nil, 0, err
	}
	return unixDir.EnumLinksAsync(fs.ctx), entries, nil
}
