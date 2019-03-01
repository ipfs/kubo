package fusemount

import (
	"errors"
	"fmt"
	"os"
	gopath "path"
	"strings"

	ds "gx/ipfs/QmUadX5EcvrBmxAV9sE7wUWtWSqxns5K84qKJBixmcT1w9/go-datastore"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	ipld "gx/ipfs/QmZ6nzCLwGLVfRzYLpD7pW6UNuBDKEcA2imJtVpbEx2rxy/go-ipld-format"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
)

const (
	invalidIndex = ^uint64(0)

	filesNamespace  = "files"
	filesRootPath   = "/" + filesNamespace
	filesRootPrefix = filesRootPath + "/"
	frs             = len(filesRootPath)
)

//TODO: remove alias
type typeToken = uint64

//TODO: cleanup
const (
	tMountRoot typeToken = iota
	tIPFSRoot
	tIPNSRoot
	tFilesRoot
	tRoots
	tIPFS
	tIPLD
	tImmutable
	tIPNS
	tIPNSKey
	tMFS
	tMutable
	tUnknown
)

func resolveMFS(filesRoot *mfs.Root, path string) (ipld.Node, error) {
	mfsNd, err := mfs.Lookup(filesRoot, path)
	if err != nil {
		return nil, err
	}
	ipldNode, err := mfsNd.GetNode()
	if err != nil {
		return nil, err
	}
	return ipldNode, nil
}

func (fs *FUSEIPFS) resolveIpns(path string) (string, error) {
	pathKey, remainder, err := ipnsSplit(path)
	if err != nil {
		return "", err
	}

	oAPI, err := fs.core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		log.Errorf("API error: %v", err)
		return "", err
	}

	var nameAPI coreiface.NameAPI
	globalPath := path
	coreKey, err := resolveKeyName(fs.ctx, oAPI.Key(), pathKey)
	switch err {
	case nil: // locally owned keys are resolved offline
		globalPath = gopath.Join(coreKey.Path().String(), remainder)
		nameAPI = oAPI.Name()

	case errNoKey: // paths without named keys are valid, but looked up via network instead of locally
		nameAPI = fs.core.Name()

	case ds.ErrNotFound: // a key was generated, but not published to / initialized
		return "", fmt.Errorf("IPNS key %q has no value", pathKey)
	default:
		return "", err
	}

	//NOTE: the returned path is not guaranteed to exist
	resolvedPath, err := nameAPI.Resolve(fs.ctx, globalPath)
	if err != nil {
		return "", err
	}
	//log.Errorf("dbg: %q -> %q -> %q", path, globalPath, resolvedPath)
	return resolvedPath.String(), nil
}

//TODO: see how IPLD selectors handle this kind of parsing
func parsePathType(path string) typeToken {
	switch {
	case path == "/":
		return tMountRoot
	case path == "/ipfs":
		return tIPFSRoot
	case path == "/ipns":
		return tIPNSRoot
	case path == filesRootPath:
		return tFilesRoot
	case strings.HasPrefix(path, "/ipld/"):
		return tIPLD
	case strings.HasPrefix(path, "/ipfs/"):
		return tIPFS
	case strings.HasPrefix(path, filesRootPrefix):
		return tMFS
	case strings.HasPrefix(path, "/ipns/"):
		if len(strings.Split(path, "/")) == 3 {
			return tIPNSKey
		}
		return tIPNS
		/* NIY
		    case strings.HasPrefix(path, "/api/"):
			return tAPI
		*/
	}

	return tUnknown
}

//operator, operator!
func parseLocalPath(path string) (fusePath, error) {
	switch parsePathType(path) {
	case tMountRoot:
		return &mountRoot{rootBase: rootBase{
			recordBase: crb("/")}}, nil
	case tIPFSRoot:
		return &ipfsRoot{rootBase: rootBase{
			recordBase: crb("/ipfs")}}, nil
	case tIPNSRoot:
		return &ipnsRoot{rootBase: rootBase{
			recordBase: crb("/ipns")}}, nil
	case tFilesRoot:
		return &mfsRoot{rootBase: rootBase{
			recordBase: crb(filesRootPath)}}, nil
	case tIPFS, tIPLD:
		return &ipfsNode{recordBase: crb(path)}, nil
	case tMFS:
		return &mfsNode{mutableBase: mutableBase{
			recordBase: crb(path)}}, nil
	case tIPNSKey:
		return &ipnsKey{ipnsNode{mutableBase: mutableBase{
			recordBase: crb(path)}}}, nil
	case tIPNS:
		return &ipnsNode{mutableBase: mutableBase{
			recordBase: crb(path)}}, nil
	case tUnknown:
		switch strings.Count(path, "/") {
		case 0:
			return nil, errors.New("invalid request")
		case 1:
			return nil, errors.New("invalid root request")
		case 2:
			return nil, errors.New("invalid root namespace")
		}
	}

	return nil, fmt.Errorf("unexpected request %q", path)
}

func crb(path string) recordBase {
	return recordBase{path: path, handles: &[]uint64{}}
}

func (fs *FUSEIPFS) parent(node fusePath) (fusePath, error) {
	if _, ok := node.(*mountRoot); ok {
		return node, nil
	}

	path := node.String()
	i := len(path) - 1
	for i != 0 && path[i] != '/' {
		i--
	}
	if i == 0 {
		return fs.LookupPath("/")
	}

	return fs.LookupPath(path[:i])
}

func (fs *FUSEIPFS) resolveToGlobal(node fusePath) (fusePath, error) {
	switch node.(type) {
	case *mfsNode:
		//contacts API node
		ipldNode, err := resolveMFS(fs.filesRoot, node.String()[frs:])
		if err != nil {
			return nil, err
		}

		if cachedNode := fs.cc.Request(ipldNode.Cid()); cachedNode != nil {
			return cachedNode, nil
		}

		//TODO: will ipld always be valid here? is there a better way to retrieve the path?
		globalNode, err := fs.LookupPath(gopath.Join("/ipld/", ipldNode.String()))
		if err != nil {
			return nil, err
		}

		fs.cc.Add(ipldNode.Cid(), globalNode)
		return globalNode, nil

	case *ipnsNode, *ipnsKey:
		resolvedPath, err := fs.resolveIpns(node.String()) //contacts API node
		if err != nil {
			return nil, err
		}

		//TODO: test if the core handles recursion protection here; IPNS->IPNS->...
		return fs.LookupPath(resolvedPath)
	}

	return nil, fmt.Errorf("unexpected reference-node type %T", node)
}

func isReference(fsNode fusePath) bool {
	switch fsNode.(type) {
	case *mfsNode, *ipnsNode, *ipnsKey:
		return true
	default:
		return false
	}
}

func isDevice(fsNode fusePath) bool {
	switch fsNode.(type) {
	case *mountRoot, *ipfsRoot, *ipnsRoot, *mfsRoot:
		return true
	default:
		return false
	}
}

//NOTE: caller should retain FS (R)Lock
func (fs *FUSEIPFS) LookupFileHandle(fh uint64) (handle *fileHandle, err error) {
	err = errInvalidHandle
	if fh == invalidIndex {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Lookup recovered from panic, likely invalid handle: %v", r)
			handle = nil
			err = errInvalidHandle
		}
	}()

	//TODO: enable when stable
	//L0 direct cast 🤠
	//return (*fileHandle)(unsafe.Pointer(uintptr(fh))), nil

	//L1 handle -> lookup -> node
	if hs, ok := fs.fileHandles[fh]; ok {
		if hs.record != nil {
			return hs, nil
		}
		//TODO: return separate error? handleInvalidated (handle was active but became bad) vs handleInvalid (never existed in the first place)
	}
	return nil, errInvalidHandle
}

//NOTE: caller should retain FS (R)Lock
func (fs *FUSEIPFS) LookupDirHandle(fh uint64) (handle *dirHandle, err error) {
	err = errInvalidHandle
	if fh == invalidIndex {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Lookup recovered from panic, likely invalid handle: %v", r)
			handle = nil
			err = errInvalidHandle
		}
	}()

	//TODO: enable when stable
	//L0 direct cast 🤠
	//return (*dirHandle)(unsafe.Pointer(uintptr(fh))), nil

	//L1 handle -> lookup -> node
	if hs, ok := fs.dirHandles[fh]; ok {
		if hs.record != nil {
			return hs, nil
		}
	}
	return nil, errInvalidHandle
}

//NOTE: caller should retain FS (R)Lock
func (fs *FUSEIPFS) LookupPath(path string) (fusePath, error) {
	if path == "" {
		return nil, errInvalidArg
	}

	//L1 path -> cid -> cache -?> record
	pathCid, err := fs.cc.Hash(path)
	if err != nil {
		log.Errorf("cache: %s", err)
	} else if cachedNode := fs.cc.Request(pathCid); cachedNode != nil {
		return cachedNode, nil
	}

	//L2 path -> full parse+construction
	parsedNode, err := parseLocalPath(path)
	if err != nil {
		return nil, err
	}

	if !isDevice(parsedNode) {
		if !fs.exists(parsedNode) {
			return parsedNode, os.ErrNotExist //NOTE: node is still a valid structure ready for use (i.e. useful for creation/type inspection)
		}
	}

	fs.cc.Add(pathCid, parsedNode)
	return parsedNode, nil
}

func (fs *FUSEIPFS) exists(parsedNode fusePath) bool {
	globalNode := parsedNode
	var err error
	if isReference(parsedNode) {

		globalNode, err = fs.resolveToGlobal(parsedNode)
		if err != nil {
			return false
		}
	}
	if _, err = fs.core.ResolvePath(fs.ctx, globalNode); err != nil { //contacts API node and possibly the network
		return false
	}

	return true
}
