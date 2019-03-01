package fusemount

import (
	"sync"
	"time"
)

//TODO: config: IPNS record-lifetime, mfs lifetime
func (fs *FUSEIPFS) dbgBackgroundRoutine() {
	var nodeType typeToken

	// invalidate cache entries
	// TODO refresh nodes in background instead?

	var wg sync.WaitGroup
	for {
		select {
		case <-fs.ctx.Done():
			log.Warning("fs context is done")
			return
		case <-time.After(10 * time.Second):
			nodeType = tIPNSKey
		case <-time.After(11 * time.Second):
			nodeType = tIPNS
		case <-time.After(2 * time.Minute):
			nodeType = tMFS
		}

		//this is likely suboptimal, quick hacks for debugging
		fs.Lock()
		wg.Add(2)
		go func() {
			defer wg.Done()
			for _, handle := range fs.fileHandles {
				path := handle.record.String()
				if parsePathType(path) == nodeType {
					fs.cc.ReleasePath(path)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for _, handle := range fs.dirHandles {
				path := handle.record.String()
				if parsePathType(path) == nodeType {
					fs.cc.ReleasePath(path)
				}
			}
		}()
		wg.Wait()
		fs.Unlock()
	}
}
