package log

import (
	"io"
	"sync"
)

type MirrorWriter struct {
	writers []io.Writer
	lk      sync.Mutex
}

func (mw *MirrorWriter) Write(b []byte) (int, error) {
	mw.lk.Lock()
	// write to all writers, and nil out the broken ones.
	var dropped bool
	for i, w := range mw.writers {
		_, err := w.Write(b)
		if err != nil {
			mw.writers[i] = nil
			dropped = true
		}
	}

	// consolidate the slice
	if dropped {
		writers := mw.writers
		mw.writers = nil
		for _, w := range writers {
			if w != nil {
				mw.writers = append(mw.writers, w)
			}
		}
	}
	mw.lk.Unlock()
	return len(b), nil
}

func (mw *MirrorWriter) AddWriter(w io.Writer) {
	mw.lk.Lock()
	mw.writers = append(mw.writers, w)
	mw.lk.Unlock()
}

func (mw *MirrorWriter) Active() (active bool) {
	mw.lk.Lock()
	active = len(mw.writers) > 0
	mw.lk.Unlock()
	return
}
