package fusemount

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unsafe"

	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"

	"github.com/billziss-gh/cgofuse/fuse"
)

const (
	tRoot typeToken = iota
	tIPNSKey
	tFAPI
	tIPFS
	tIPLD
	tIPNS
	tUnknown
)

func parsePathType(path string) typeToken {

	slashCount := len(strings.Split(path, "/"))
	switch {
	case slashCount == 1: // `/`, `/ipfs`, ...
		return tRoot
	case strings.HasPrefix(path, "/ipld/"):
		return tIPLD
	case strings.HasPrefix(path, "/ipfs/"):
		return tIPFS
	case strings.HasPrefix(path, filesRootPrefix):
		return tFAPI
	case strings.HasPrefix(path, "/ipns/"):
		return tIPNS
	}

	return tUnknown
}

func parseFusePath(fs *FUSEIPFS, subsystemType typeToken, path string) (fusePath, error) {
	switch subsystemType {
	case tIPFS, tIPLD:
		return &ipfsNode{recordBase: crb(path)}, nil
	case tFAPI:
		return &filesAPINode{mfsNode{root: fs.filesRoot, recordBase: crb(path[len(filesRootPath):])}}, nil
	case tIPNS:
		nn := &ipnsNode{ipfsNode: ipfsNode{recordBase: crb(path)}}

		//NOTE: path is assumed valid because of tIPNS from previous parser
		keyComponent := strings.Split(path, "/")[2]
		index := strings.Index(path, keyComponent) + len(keyComponent)
		subPath := path[index:]

		callContext, cancel := deriveCallContext(fs.ctx)
		defer cancel()
		var err error
		switch nn.key, err = resolveKeyName(callContext, fs.core.Key(), keyComponent); err {
		case nil: // key is owned and found, use mfs methods internally for write access
			//nn.ipfsNode.recordBase.path = subPath // mfs expects paths relative to its own root
			//nn.subsystem = nn.mfsNode
		case errNoKey:
			// non-owned keys are valid, but read only
			//nn.subsystem = nn.ipfsNode
			break
		default:
			return nil, err
		}

		if nn.key != nil && len(subPath) == 0 { // promote `/ipns/ownedKey` to subroot "/"
			//nn.path = path[:1]
			return &ipnsSubroot{ipnsNode: *nn}, nil
		}
		return nn, nil

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
	return recordBase{path: path, ioHandles: make(nodeHandles)}
}

//FIXME: implicit locks
//NOTE: caller should retain FS (R)Lock
func (fs *FUSEIPFS) shallowLookupPath(path string) (fusePath, error) {
	if path == "" {
		return nil, errInvalidArg
	}

	//L1 path -> cid -> cache -?> record
	pathCid, err := fs.cc.Hash(path)
	if err != nil {
		log.Errorf("cache: %s", err) //TODO: remove; debug
	} else if cachedNode := fs.cc.Request(pathCid); cachedNode != nil {
		//TODO: check record TTL conditions here
		//FIXME if expired, reset IO, drop through to parse logic
		return cachedNode, nil
	}

	//L2 check open handles for active paths
	//TODO/FIXME: important; shared mutex is required

	//L3 parse string path, construct typed-node
	var parsedNode fusePath
	switch apiType := parsePathType(path); apiType {
	case tRoot:
		if parsedNode, err = parseRootPath(fs, path); err != nil {
			return nil, err
		}
	default:
		if parsedNode, err = parseFusePath(fs, apiType, path); err != nil {
			return nil, err
		}

	case tUnknown:
		return nil, errUnexpected
	}

	// populate node's required data
	callContext, cancel := deriveCallContext(fs.ctx)
	defer cancel()
	if nodeStat, err := parsedNode.InitMetadata(callContext); err != nil {
		if err == os.ErrNotExist { // NOTE: non-existent nodes are still valid for creation operations
			return parsedNode, err
		}
		return nil, err
	}

	fs.cc.Add(pathCid, parsedNode)
	return parsedNode, err
}

const linkRecurseLimit = 255 //FIXME: arbitrary debug value
func (fs *FUSEIPFS) LookupPath(path string) (fsNode fusePath, err error) {
	targetPath := path
	for depth := 0; depth != linkRecurseLimit; depth++ {
		fsNode, err = fs.shallowLookupPath(targetPath)
		if err != nil {
			return
		}
		var nodeStat *fuse.Stat_t
		nodeStat, err = fsNode.Stat()
		if err != nil {
			return
		}

		if nodeStat.Mode&fuse.S_IFMT != fuse.S_IFLNK {
			return
		}

		// if node is link, resolve to its target
		var targetNode fusePath
		callContext, cancel := deriveCallContext(fs.ctx)
		defer cancel()
		ioIf, ioErr := fsNode.YieldIo(callContext, unixfs.TSymlink)
		if ioErr != nil {
			err = ioErr
			return
		}

		targetPath := ioIf.(FsLink).Target()
	}
	err = errRecurse
	return
}

func invertedLookup(fh uint64) (fp fusePath, io interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("invertedLookup recovered from panic, likely invalid handle: %v", r)
			fp = nil
			io = nil
			err = errInvalidHandle
		}
	}()

	if io, ok := (*(*interface{})(unsafe.Pointer(uintptr(fh)))).(FsRecord); ok {
		fp = io.Record()
		return fp, io, nil
	}
	err = errUnexpected
	return
}

/*
func updateStale(ctx context.Context, fsNode fusePath) error {
	//check if node.metadata == stale
	/* if is
	   store type bits
	   re-init with Stat()
	   if non-exist or type bits changed; return err
	   for each node handle
	   fh = fsNode.YieldIo(unixType)
}
*/
