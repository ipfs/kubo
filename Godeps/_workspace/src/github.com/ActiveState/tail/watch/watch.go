// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

package watch

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/gopkg.in/tomb.v1"
	"os"
)

// FileWatcher monitors file-level events.
type FileWatcher interface {
	// BlockUntilExists blocks until the file comes into existence.
	BlockUntilExists(*tomb.Tomb) error

	// ChangeEvents reports on changes to a file, be it modification,
	// deletion, renames or truncations. Returned FileChanges group of
	// channels will be closed, thus become unusable, after a deletion
	// or truncation event.
	ChangeEvents(*tomb.Tomb, os.FileInfo) *FileChanges
}
