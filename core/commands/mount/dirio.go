package fusemount

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"

	coreiface "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core"
	coreoptions "gx/ipfs/QmXLwxifxwfc2bAwq6rdjbYqAsGzWsDE9RM5TWMGtykyj6/interface-go-ipfs-core/options"
	mfs "gx/ipfs/Qmb74fRYPgpjYzoBV7PAVNmP3DQaRrh8dHdKE4PwnF3cRx/go-mfs"
	unixfs "gx/ipfs/QmcYUTQ7tBZeH1CLsZM2S3xhMEZdvUgXvbjhpMsLDpk3oJ/go-unixfs"
)

//TODO: rename? directoryStreamHandle
type directoryStream struct {
	record  fusePath
	entries []directoryEntry
	err     error
	init    *sync.Cond

	//initialize sync.Once
	//sem        sync.Cond
	//stream     <-chan DirectoryMessage
}

//TODO: better name; AsyncDirEntry?
// API format bridge
type DirectoryMessage struct {
	directoryEntry
	error
}

//NOTE: purpose is to be cached and updated when needed instead of re-initialized each call for path; thread safe as a consequent
type directoryEntry struct {
	//sync.RWMutex
	label string
	stat  *fuse.Stat_t
}

func (fs *FUSEIPFS) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) int {
	fs.RLock()
	log.Debugf("Readdir - Request +%d[%X]%q", ofst, fh, path)
	if ofst < 0 {
		fs.RUnlock()
		return -fuse.EINVAL
	}

	dh, err := fs.LookupDirHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Readdir - [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}
	dh.record.RLock()
	fs.RUnlock()
	defer dh.record.RUnlock()

	if dh.io.Entries() == 0 {
		//TODO: reconsider/discuss this behaviour; dots are not actually required in POSIX or FUSE
		// not having them on empty directories causes things like `dir` and `ls` to fail since they look for the dot, but maybe they should since IPFS does not store dot entries in its directories
		fill(".", dh.record.Stat(), -1)
		return fuseSuccess
	}

	for {
		select {
		case <-fs.ctx.Done():
			return -fuse.EBADF
		case msg := <-dh.io.Read(ofst):
			err = msg.error
			label := msg.directoryEntry.label
			stat := msg.directoryEntry.stat

			if err != nil {
				if err == io.EOF {
					fill(label, stat, -1)
					return fuseSuccess
				}
				log.Errorf("Readdir - %q list err: %s", path, err)
				return -fuse.ENOENT
			}

			ofst++
			if !fill(label, stat, ofst) {
				return fuseSuccess
			}
		}
	}
}

func (ds *directoryStream) Read(ofst int64) <-chan DirectoryMessage {
	msgChan := make(chan DirectoryMessage, 1)
	if int64(cap(ds.entries)) <= ofst {
		msgChan <- DirectoryMessage{error: fmt.Errorf("invalid offset %d <= %d", cap(ds.entries), ofst)}
		return msgChan
	}

	if ds.init != nil {
		ds.init.L.Lock()
		for int64(len(ds.entries)) < ofst || ds.err == nil {
			ds.init.Wait()
		}

		switch ds.err {
		case io.EOF:
			ds.init = nil
			ds.err = nil
		default:
			msgChan <- DirectoryMessage{error: ds.err}
			return msgChan
		}
	}

	// NOTE: it it assumed that if len(entries) == ofst and EOF is set, that the entry slot is populated
	// if the spooler lies, we'll likely panic
	// index change 0 -> 1
	if ofst+1 == int64(len(ds.entries)) {
		msgChan <- DirectoryMessage{directoryEntry: ds.entries[ofst], error: io.EOF}
	} else {
		msgChan <- DirectoryMessage{directoryEntry: ds.entries[ofst]}
	}

	return msgChan
}

//TODO: [readdirplus] stats
//TODO: name: pulse -> timeoutExtender? signalCallback? entryEvent?
//TODO: document/handle this is only intended to be used with non-empty directories; length must be checked by caller
func (ds *directoryStream) spool(ctx context.Context, inStream <-chan DirectoryMessage, pulse func()) {
	defer pulse()
	entCap := cap(ds.entries)
	for i := 0; i != entCap; i++ {
		select {
		case <-ctx.Done():
			return
		case msg := <-inStream:
			if msg.error != nil {
				ds.err = msg.error
				return
			}

			if msg.directoryEntry.label == "" {
				ds.err = fmt.Errorf("directory contains empty entry label")
				return
			}

			if fReaddirPlus && msg.directoryEntry.stat == nil {
				ds.err = fmt.Errorf("Readdir - stat for %q is not initialized", msg.directoryEntry.label)
				return
			}

			ds.entries = append(ds.entries, msg.directoryEntry)
			pulse()
		}
	}
	ds.err = io.EOF
}

func (ds *directoryStream) Entries() int {
	return cap(ds.entries)
}

func coreMux(cc <-chan coreiface.LsLink) <-chan DirectoryMessage {
	msgChan := make(chan DirectoryMessage)
	go func() {
		for m := range cc {
			ent := directoryEntry{label: m.Link.Name}
			msgChan <- DirectoryMessage{directoryEntry: ent, error: m.Err}
		}
		close(msgChan)
	}()
	return msgChan
}

func mfsMux(uc <-chan unixfs.LinkResult) <-chan DirectoryMessage {
	msgChan := make(chan DirectoryMessage)
	go func() {
		for m := range uc {
			ent := directoryEntry{label: m.Link.Name}
			msgChan <- DirectoryMessage{directoryEntry: ent, error: m.Err}
		}
		close(msgChan)
	}()
	return msgChan
}

//TODO:
func activeDir(fsNode fusePath) FsDirectory {
	//TODO: if fsNode.Handles(); ret handles[0].dirio
	return nil
}

func (fs *FUSEIPFS) yieldDirIO(fsNode fusePath) (FsDirectory, error) {
	dirStream := &directoryStream{record: fsNode}
	switch fsNode.(type) {
	default:
		return nil, errors.New("unexpected type")

	case *mountRoot:
		dirStream.entries = fs.roots

	case *ipfsRoot:
		pins, err := fs.core.Pin().Ls(fs.ctx, coreoptions.Pin.Type.Recursive())
		if err != nil {
			log.Errorf("ipfsRoot - Ls err: %v", err)
			return nil, err
		}

		ents := make([]directoryEntry, 0, len(pins))
		for _, pin := range pins {
			//TODO: [readdirplus] stats
			ents = append(ents, directoryEntry{label: pin.Path().Cid().String()})
		}
		dirStream.entries = ents

	case *ipnsRoot:
		dirStream.entries = fs.ipnsRootSubnodes()
	}

	return dirStream, nil
}

func (fs *FUSEIPFS) yieldAsyncDirIO(ctx context.Context, timeoutGrace time.Duration, fsNode fusePath) (FsDirectory, error) {
	if cached := activeDir(fsNode); cached != nil {
		return cached, nil
	}

	var inputChan <-chan DirectoryMessage
	var entryCount int

	switch fsNode.(type) {
	default:
		return nil, errors.New("unexpected type")
	case *ipfsNode, *ipnsNode:
		globalNode := fsNode
		if isReference(fsNode) {
			var err error
			globalNode, err = fs.resolveToGlobal(fsNode)
			if err != nil {
				return nil, err
			}
		}

		//TODO: [readdirplus] stats
		coreChan, err := fs.core.Unixfs().Ls(ctx, globalNode, coreoptions.Unixfs.ResolveChildren(false))
		if err != nil {
			return nil, err
		}

		oStat, err := fs.core.Object().Stat(ctx, globalNode)
		if err != nil {
			return nil, err
		}

		entryCount = oStat.NumLinks
		if entryCount != 0 {
			inputChan = coreMux(coreChan)
		}

	case *mfsNode, *mfsRoot, *ipnsKey:
		var ( //XXX: there's probably a better way to handle this; go prevents a nice fallthrough
			mRoot *mfs.Root
			mPath string
		)
		if _, ok := fsNode.(*ipnsKey); ok {
			var err error
			if mRoot, mPath, err = fs.ipnsMFSSplit(fsNode.String()); err != nil {
				return nil, err
			}
		} else {
			mRoot = fs.filesRoot
			mPath = fsNode.String()[frs:]
		}

		mfsChan, count, err := fs.mfsSubNodes(mRoot, mPath)
		if err != nil {
			return nil, err
		}
		entryCount = count
		if entryCount != 0 {
			inputChan = mfsMux(mfsChan)
		}
	}

	//TODO: move this to doc.go or something
	/* Opendir() -> .spool() -> Readdir() real-time, cross thread synchronization
	- .init: use to .init.Wait() in reader, until entry slot N or .err is populated
		set .init to nil in waiting thread after processing error|EOF
	- .spool(): responsible for populating directory entries list and directory's error value
		set .err to io.EOF when finished without errors
	- timeout.AfterFunc(): responsible for reporting timeout to directory,
		cleaning up resources, and sending wake-up event.
		Basically an object with this capability:
		context.WithDeadline(deadline, func()).Reset(duration)
	- pulser(): responsible for extending timeout and sending wake-up event
	*/

	backgroundDir := &directoryStream{record: fsNode, entries: make([]directoryEntry, 0, entryCount)}
	if entryCount == 0 {
		backgroundDir.err = io.EOF
		return backgroundDir, nil
	}

	backgroundDir.init = sync.NewCond(&sync.Mutex{})
	callContext, cancel := context.WithCancel(ctx)
	cancelClosure := func() {
		if backgroundDir.err == nil { // don't overwrite error if it existed before the timeout
			backgroundDir.err = errors.New("timed out")
		}
		cancel()
	}

	timeout := time.AfterFunc(timeoutGrace, cancelClosure)
	pulser := func() {
		defer backgroundDir.init.Broadcast()
		if backgroundDir.err != nil {
			return
		}

		if !timeout.Stop() {
			<-timeout.C
		}
		timeout.Reset(timeoutGrace)
	}

	backgroundDir.spool(callContext, inputChan, pulser)

	return backgroundDir, nil
}
