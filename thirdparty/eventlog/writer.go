package eventlog

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
	var filter []io.Writer
	for _, w := range mw.writers {
		_, err := w.Write(b)
		if err == nil {
			filter = append(filter, w)
		}
	}
	mw.writers = filter
	mw.lk.Unlock()
	return len(b), nil
}

func (mw *MirrorWriter) AddWriter(w io.Writer) {
	mw.lk.Lock()
	mw.writers = append(mw.writers, w)
	mw.lk.Unlock()
}
