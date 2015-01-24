// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package watch

import (
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/ActiveState/tail/util"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/howeyc/fsnotify"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/gopkg.in/tomb.v1"
	"os"
	"path/filepath"
)

var inotifyTracker *InotifyTracker

// InotifyFileWatcher uses inotify to monitor file changes.
type InotifyFileWatcher struct {
	Filename string
	Size     int64
}

func NewInotifyFileWatcher(filename string) *InotifyFileWatcher {
	fw := &InotifyFileWatcher{filename, 0}
	return fw
}

func (fw *InotifyFileWatcher) BlockUntilExists(t *tomb.Tomb) error {
	w, err := inotifyTracker.NewWatcher()
	if err != nil {
		return err
	}
	defer inotifyTracker.CloseWatcher(w)

	dirname := filepath.Dir(fw.Filename)

	// Watch for new files to be created in the parent directory.
	err = w.WatchFlags(dirname, fsnotify.FSN_CREATE)
	if err != nil {
		return err
	}

	// Do a real check now as the file might have been created before
	// calling `WatchFlags` above.
	if _, err = os.Stat(fw.Filename); !os.IsNotExist(err) {
		// file exists, or stat returned an error.
		return err
	}

	for {
		select {
		case evt, ok := <-w.Event:
			if !ok {
				return fmt.Errorf("inotify watcher has been closed")
			} else if evt.Name == fw.Filename {
				return nil
			}
		case <-t.Dying():
			return tomb.ErrDying
		}
	}
	panic("unreachable")
}

func (fw *InotifyFileWatcher) ChangeEvents(t *tomb.Tomb, fi os.FileInfo) *FileChanges {
	changes := NewFileChanges()

	w, err := inotifyTracker.NewWatcher()
	if err != nil {
		util.Fatal("Error creating fsnotify watcher: %v", err)
	}
	err = w.Watch(fw.Filename)
	if err != nil {
		util.Fatal("Error watching %v: %v", fw.Filename, err)
	}

	fw.Size = fi.Size()

	go func() {
		defer inotifyTracker.CloseWatcher(w)
		defer changes.Close()

		for {
			prevSize := fw.Size

			var evt *fsnotify.FileEvent
			var ok bool

			select {
			case evt, ok = <-w.Event:
				if !ok {
					return
				}
			case <-t.Dying():
				return
			}

			switch {
			case evt.IsDelete():
				fallthrough

			case evt.IsRename():
				changes.NotifyDeleted()
				return

			case evt.IsModify():
				fi, err := os.Stat(fw.Filename)
				if err != nil {
					if os.IsNotExist(err) {
						changes.NotifyDeleted()
						return
					}
					// XXX: report this error back to the user
					util.Fatal("Failed to stat file %v: %v", fw.Filename, err)
				}
				fw.Size = fi.Size()

				if prevSize > 0 && prevSize > fw.Size {
					changes.NotifyTruncated()
				} else {
					changes.NotifyModified()
				}
				prevSize = fw.Size
			}
		}
	}()

	return changes
}

func Cleanup() {
	inotifyTracker.CloseAll()
}

func init() {
	inotifyTracker = NewInotifyTracker()
}
