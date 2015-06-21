package files

import (
	"os"
	"time"
)

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func NewDummyRegularFileInfo(name string) (fi os.FileInfo) {
	return &fileInfo{
		name:    name,
		size:    0,
		mode:    os.ModePerm,
		modTime: time.Time{},
	}
}

func NewDummyDirectoryFileInfo(name string) (fi os.FileInfo) {
	return &fileInfo{
		name:    name,
		size:    0,
		mode:    os.ModePerm | os.ModeDir,
		modTime: time.Time{},
	}
}

func (fi *fileInfo) Name() string {
	return fi.name
}

func (fi *fileInfo) Size() int64 {
	return fi.size
}

func (fi *fileInfo) Mode() os.FileMode {
	return fi.mode
}

func (fi *fileInfo) ModTime() time.Time {
	return fi.modTime
}

func (fi *fileInfo) IsDir() bool {
	return (fi.mode & os.ModeDir) != 0
}

func (fi *fileInfo) Sys() interface{} {
	return nil
}
