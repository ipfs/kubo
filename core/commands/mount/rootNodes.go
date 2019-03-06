package fusemount

import (
	"context"
	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/ipfs/go-ipfs/core/coreapi"
)

//TODO: document registering with this
type softDirRoot struct {
	recordBase
	//fs *FUSEIPFS
}

type mountRoot struct {
	softDirRoot
	subroots []string
}

type pinRoot struct {
	softDirRoot
	pinAPI coreapi.PinAPI
}

type keyRoot struct {
	softDirRoot
	keyAPI coreiface.KeyAPI
}

func (sd *softDirRoot) InitMetadata(_ context.Context) (*fuse.Stat_t, error) {
	sd.recordBase.metadata.Mode = fuse.S_IFDIR | IRXA
	return &sd.recordBase.metadata, nil
}

func (kr *keyRoot) InitMetadata(ctx context.Context) (*fuse.Stat_t, error) {
	nodeStat, err := kr.softDirRoot.InitMetadata(ctx)
	if err != nil {
		return nodeStat, err
	}
	nodeStat.Mode |= fuse.S_IWUSR
	return nodeStat, nil
}

func (pr *pinRoot) YieldIo(ctx context.Context, nodeType FsType) (io interface{}, err error) {
	if err := in.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	pins, err := pr.pinAPI.Ls(ctx, coreoptions.Pin.Type.Recursive())
	if err != nil {
		return nil, err
	}

	pinChan := make(chan directoryStringEntry)
	asyncContext := deriveTimerContext(ctx, entryTimeout)
	go func() {
		defer close(pinChan)
		for _, pin := range pins {
			select {
			case <-asyncContext.Done():
				return
			case pinChan <- directoryStringEntry{string: pin.String()}:
				continue
			}
		}

	}()
	return backgroundDir(asyncContext, len(pins), pinChan)
}

func (kr *keyRoot) YieldIo(ctx context.Context, nodeType FsType) (io interface{}, err error) {
	if err := in.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	keys, err := kr.keyAPI.List(ctx)
	if err != nil {
		return nil, err
	}

	keyChan := make(chan directoryStringEntry)
	asyncContext := deriveTimerContext(ctx, entryTimeout)
	go func() {
		defer close(keyChan)
		for _, key := range keys {
			select {
			case <-asyncContext.Done():
				return
			case keyChan <- directoryStringEntry{string: key.Name()}:
				continue
			}
		}

	}()
	return backgroundDir(asyncContext, len(keys), keyChan)
}

func (mr *mountRoot) YieldIo(ctx context.Context, nodeType FsType) (io interface{}, err error) {
	if err := in.recordBase.typeCheck(nodeType); err != nil {
		return nil, err
	}

	rootChan := make(chan directoryStringEntry)
	asyncContext := deriveTimerContext(ctx, entryTimeout)
	go func() {
		defer close(rootChan)
		for _, subroot := range mr.subroots {
			select {
			case <-asyncContext.Done():
				return
			case rootChan <- directoryStringEntry{string: subroot}:
				continue
			}
		}
	}()
	return backgroundDir(asyncContext, len(mr.subroots), rootChan)
}

/*
mountroot: entries: func() lookup([]static-stringl-list)
*/

func parseRootPath(fs *FUSEIPFS, path string) (fusePath, error) {
	//pass in root via context on init
	//use root on object self to reinit self
	switch path {
	case filesRootPrefix:
		return mfsNode{root: fs.filesRoot}, nil
	case "/ipns":
		return &keyRoot{keyAPI: fs.core.Key(), softDirRoot: csd(path, fs.mountTime)}, nil
	case "/ipfs":
		return &pinRoot{pinAPI: fs.core.Pin(), softDirRoot: csd(path, fs.mountTime)}, nil
	case "/":
		return &mountRoot{subroots: []string{"/ipfs", "/ipns", filesRootPrefix},
			csd(path, fs.mountTime)}, nil
	}
}

func csd(path string, now fuse.Timespec) softDirRoot {
	sd := softDirRoot{recordBase: crb(path)}
	meta := &sd.recordBase.metadata
	meta.Birthtim, meta.Atim, meta.Mtim, meta.Ctim = now, now, now, now
	return sd
}
