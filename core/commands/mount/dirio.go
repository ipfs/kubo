package fusemount

import (
	"context"
	"errors"
	"fmt"
	"io"
	gopath "path"
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
	record fusePath
	//entries []directoryEntry
	entries []fusePath
	err     error
	init    *sync.Cond

	//initialize sync.Once
	//sem        sync.Cond
	//stream     <-chan DirectoryMessage
}

// API format bridges
type directoryStringEntry struct {
	string
	error
}
type directoryFuseEntry struct {
	fusePath
	error
}

func (ds *directoryStream) Record() fusePath {
	return ds.record
}

func (ds *directoryStream) Lock() {
	//TODO: figure out who calls this; I think we need it for re-init of shared array
	ds.record.Lock()
}

func (ds *directoryStream) Unlock() {
	ds.record.Unlock()
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

	ioIf, err := fs.getIo(fh, unixfs.TDirectory)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Readdir - [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}
	dio := ioIf.(FsDirectory)
	dio.Lock()
	defer dio.Unlock()
	fs.RUnlock()

	//NOTE: dots are not required in the standard; we only fill them for compatibility with programs that don't expect truly empty directories
	if dio.Entries() == 0 {
		fStat, err := dio.Record().Stat()
		if err != nil {
			log.Errorf("Readdir - %q can't fetch metadata: %s", path, err)
			return -fuse.EBADF // node must be invalid in some way
		}
		fill(".", fStat, -1)
		return fuseSuccess
	}

	ctx, cancel := context.WithCancel(fs.ctx)
	defer cancel()
	for msg := range dio.Read(ctx, ofst) {
		if msg.error != nil && msg.error != io.EOF {
			log.Errorf("Readdir - %q entry err: %s", path, msg.error)
			return -fuse.ENOENT
		}

		label := gopath.Base(msg.fusePath.String())
		fStat, err := msg.fusePath.Stat()
		if err != nil {
			log.Errorf("Readdir - %q can't fetch metadata for %q: %s", path, label, err)
			return -fuse.ENOENT
		}

		ofst++
		if !fill(label, fStat, ofst) {
			return fuseSuccess // fill signaled an early exit
		}
	}
	return fuseSuccess
}

func (ds *directoryStream) Read(ctx context.Context, ofst int64) <-chan directoryFuseEntry {
	msgChan := make(chan directoryFuseEntry, 1)
	entCap := cap(ds.entries)
	if int(ofst) > entCap {
		msgChan <- directoryFuseEntry{error: fmt.Errorf("invalid offset:%d entries:%d", ofst, entCap)}
		return msgChan
	}

	//FIXME: we need to be able to cancel this
	go func() {
	out:
		for i := int(ofst); i != entCap; i++ {
			if ds.err != io.EOF { // directory isn't fully populated
				ds.init.L.Lock()
				for len(ds.entries) < i || ds.err == nil {
					ds.init.Wait() // wait until offset is populated or error
				}

				switch ds.err {
				case io.EOF:
					// all entries are ready; init is nil
					break
				case nil:
					// entry at offset is ready, but spool is not finished yet
					ds.init.L.Unlock()
					break
				default:
					// spool reported an error
					ds.init.L.Unlock()
					msgChan <- directoryFuseEntry{error: ds.err}
					return
				}
			}

			msg := directoryFuseEntry{fusePath: ds.entries[i-1]}
			if i == entCap {
				msg.error = io.EOF
			}

			select {
			case <-ctx.Done():
				msg.error = ctx.Err()
				msgChan <- msg
				break out
			case msgChan <- msg:
				continue
			}
		}
		close(msgChan)
	}()
	return msgChan
}

//TODO: name: pulse -> timeoutExtender? signalCallback? entryEvent?
func (ds *directoryStream) spool(tctx timerContext, inStream <-chan directoryStringEntry, pulse func()) {
	defer tctx.Cancel()
	defer func() {
		if ds.err != io.EOF {
			pulse()
		}
	}()

	lf := tctx.Value(lookupKey{})
	if lf == nil {
		ds.err = fmt.Errorf("lookup function not provided (via context) to directory spooler")
		return
	}
	lookup, ok := lf.(lookupFn)
	if !ok {
		ds.err = fmt.Errorf("provided lookup function does not match signature")
		return
	}

	var wg sync.WaitGroup
	entCap := cap(ds.entries)
	for i := 0; i != entCap; i++ {
		select {
		case <-tctx.Done():
			ds.err = tctx.Err()
			return
		case msg := <-inStream:
			if msg.error != nil {
				ds.err = msg.error
				return
			}

			label := msg.string

			if label == "" {
				ds.err = fmt.Errorf("directory contains empty entry label")
				return
			}

			wg.Add(1) // fan out
			go func(i int) {
				defer wg.Done()
				fsNode, err := lookup(label)
				if err != nil {
					ds.err = fmt.Errorf("directory entry %q lookup err: %s", label, err)
					return
				}
				ds.init.L.Lock()
				ds.entries = append(ds.entries, fsNode)
				if i == entCap-1 {
					ds.err = io.EOF
				}
				ds.init.L.Unlock()
				pulse()
			}(i)
		}
	}
	wg.Wait()
	ds.init = nil
}

func (ds *directoryStream) Entries() int {
	return cap(ds.entries)
}

func coreMux(cc <-chan coreiface.LsLink) <-chan directoryStringEntry {
	msgChan := make(chan directoryStringEntry)
	go func() {
		for m := range cc {
			msgChan <- directoryStringEntry{string: m.Link.Name, error: m.Err}
		}
		close(msgChan)
	}()
	return msgChan
}

func mfsMux(uc <-chan unixfs.LinkResult) <-chan directoryStringEntry {
	msgChan := make(chan directoryStringEntry)
	go func() {
		for m := range uc {
			msgChan <- directoryStringEntry{string: m.Link.Name, error: m.Err}
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

func mfsYieldDirIO(ctx context.Context, mRoot *mfs.Root, mPath string, entryTimeout time.Duration) (FsDirectory, error) {
	//NOTE: this special timeout context is extended in backgroundDir
	asyncContext := deriveTimerContext(ctx, entryTimeout)

	mfsChan, entryCount, err := mfsSubNodes(asyncContext, mRoot, mPath)
	if err != nil {
		return nil, err
	}

	return backgroundDir(asyncContext, entryCount, mfsChan)
}

func coreYieldDirIO(ctx context.Context, corePath coreiface.Path, core coreiface.CoreAPI, entryTimeout time.Duration) (FsDirectory, error) {
	callContext, cancel := deriveCallContext(ctx)
	oStat, err := core.Object().Stat(callContext, corePath)
	if err != nil {
		cancel()
		return nil, err
	}
	cancel()

	//NOTE: this special timeout context is extended in backgroundDir
	asyncContext := deriveTimerContext(ctx, entryTimeout)
	coreChan, err := core.Unixfs().Ls(asyncContext, corePath, coreoptions.Unixfs.ResolveChildren(false))
	if err != nil {
		return nil, err
	}

	return backgroundDir(asyncContext, oStat.NumLinks, coreMux(coreChan))
}

//TODO: refine docs
/* backgroundDir documentation, real-time based failure, intended to be initialized once in the background and shared, reset upon change
- .init: use to .init.Wait() in reader, until entry slot N or .err is populated
	if .err == io.EOF then .init == nil and should not be used (even if Wait() was previously called)
- .spool(): responsible for populating .entries and .err
	set .err to io.EOF when all entries are processed
- entryCallback(): called after entry is processed and immediately before .init is freed
*/
func backgroundDir(tctx timerContext, entryCount int, inputChan <-chan directoryStringEntry) (FsDirectory, error) {
	backgroundDir := &directoryStream{entries: make([]fusePath, 0, entryCount)}
	if entryCount == 0 {
		backgroundDir.err = errors.New("empty directory stream")
		tctx.Cancel()
		return backgroundDir, nil
	}

	backgroundDir.init = sync.NewCond(&sync.Mutex{})
	pulser := func() {
		tctx.Reset()                   // extend context timeout
		backgroundDir.init.Broadcast() // wake up .Read() if it's waiting
	}

	backgroundDir.spool(tctx, inputChan, pulser)

	return backgroundDir, nil
}
