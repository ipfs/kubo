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
	for i, w := range mw.writers {
		_, err := w.Write(b)
		if err != nil {
			mw.writers[i] = nil
		}
	}

	// consolidate the slice
	for i := 0; i < len(mw.writers); i++ {
		if mw.writers[i] != nil {
			continue
		}

		j := len(mw.writers) - 1
		for ; j > i; j-- {
			if mw.writers[j] != nil {
				mw.writers[i], mw.writers[j] = mw.writers[j], nil // swap
				break
			}
		}
		mw.writers = mw.writers[:j]
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
