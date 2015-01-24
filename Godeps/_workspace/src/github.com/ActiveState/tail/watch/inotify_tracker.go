// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package watch

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/howeyc/fsnotify"
	"log"
	"sync"
)

type InotifyTracker struct {
	mux      sync.Mutex
	watchers map[*fsnotify.Watcher]bool
}

func NewInotifyTracker() *InotifyTracker {
	t := new(InotifyTracker)
	t.watchers = make(map[*fsnotify.Watcher]bool)
	return t
}

func (t *InotifyTracker) NewWatcher() (*fsnotify.Watcher, error) {
	t.mux.Lock()
	defer t.mux.Unlock()
	w, err := fsnotify.NewWatcher()
	if err == nil {
		t.watchers[w] = true
	}
	return w, err
}

func (t *InotifyTracker) CloseWatcher(w *fsnotify.Watcher) (err error) {
	t.mux.Lock()
	defer t.mux.Unlock()
	if _, ok := t.watchers[w]; ok {
		err = w.Close()
		delete(t.watchers, w)
	}
	return
}

func (t *InotifyTracker) CloseAll() {
	t.mux.Lock()
	defer t.mux.Unlock()
	for w, _ := range t.watchers {
		if err := w.Close(); err != nil {
			log.Printf("Error closing watcher: %v", err)
		}
		delete(t.watchers, w)
	}
}
