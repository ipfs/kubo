package fusemount

import (
	"io"

	"github.com/billziss-gh/cgofuse/fuse"
)

//TODO: [cleanup] move to another source file; no need for single function
//TODO: set c|mtim; see note on Read()
func (fs *FUSEIPFS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	fs.RLock()
	log.Debugf("Write - Request {%d}[%X]%q", ofst, fh, path)

	if ofst < 0 {
		log.Errorf("Write - Invalid offset {%d}[%X]%q", ofst, fh, path)
		fs.RUnlock()
		return -fuse.EINVAL
	}

	h, err := fs.LookupFileHandle(fh)
	if err != nil {
		fs.RUnlock()
		log.Errorf("Write - Lookup failed [%X]%q: %s", fh, path, err)
		if err == errInvalidHandle {
			return -fuse.EBADF
		}
		return -fuse.EIO
	}
	h.record.Lock()
	fs.RUnlock()
	defer h.record.Unlock()
	fStat := h.record.Stat()

	oCur, err := h.io.Seek(0, io.SeekCurrent)
	if err != nil {
		log.Errorf("Write - cursor error: %s", err)
		return -fuse.EIO
	}

	written, err := h.io.Write(buff, ofst)
	if err != nil {
		log.Errorf("Write - error %q: %s", path, err)
		/* Callers responsibility?
		if err = pIo.Close(); err != nil {
			log.Errorf("Write - Close error %s", err)
		}
		*/
		return -fuse.EIO
	}

	if oCur+int64(written) > fStat.Size { // write extended file
		fStat.Size += int64(written)
	}

	// when the same path has multiple open handles we need to swap the IO for them
	errPair := fs.refreshFileSiblings(fh, h)
	if len(errPair) != 0 {
		for _, e := range errPair {
			log.Errorf("Write - handle update failed for %X: %s", e.fhi, e.err)
		}
		return -fuse.EIO
	}

	return written
}
