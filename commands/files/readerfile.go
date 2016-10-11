package files

import (
	"errors"
	"io"
	"os"
)

// ReaderFile is a implementation of File created from an `io.Reader`.
// ReaderFiles are never directories, and can be read from and closed.
type ReaderFile struct {
	filename string
	fullpath string
	reader   io.ReadCloser
	stat     os.FileInfo
	root     bool
}

func NewReaderFile(filename, path string, reader io.ReadCloser, stat os.FileInfo) *ReaderFile {
	return &ReaderFile{
		filename: filename,
		fullpath: path,
		reader:   reader,
		stat:     stat,
	}
}

func (f *ReaderFile) IsDirectory() bool {
	return false
}

func (f *ReaderFile) NextFile() (File, error) {
	return nil, ErrNotDirectory
}

func (f *ReaderFile) FileName() string {
	return f.filename
}

func (f *ReaderFile) FullPath() string {
	return f.fullpath
}

func (f *ReaderFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *ReaderFile) Close() error {
	return f.reader.Close()
}

func (f *ReaderFile) Stat() os.FileInfo {
	return f.stat
}

func (f *ReaderFile) Size() (int64, error) {
	if f.stat == nil {
		return 0, errors.New("File size unknown")
	}
	return f.stat.Size(), nil
}

func (f *ReaderFile) IsRoot() bool {
	return f.root
}
