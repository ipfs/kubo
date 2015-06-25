package files

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

// ReaderFile is a implementation of File created from an `io.Reader`.
// ReaderFiles are never directories, and can be read from and closed.
type ReaderFile struct {
	filename string
	reader   io.ReadCloser
	stat     os.FileInfo
}

func NewReaderFile(filename string, reader io.ReadCloser, stat os.FileInfo) *ReaderFile {
	return &ReaderFile{filename, reader, stat}
}

func NewSymlink(path string) (File, error) {
	stat, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	target, err := os.Readlink(path)
	if err != nil {
		return nil, err
	}
	reader := strings.NewReader(target)
	readCloser := ioutil.NopCloser(reader)
	return &ReaderFile{path, readCloser, stat}, nil
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

func (f *ReaderFile) Read(p []byte) (int, error) {
	if f.reader == nil {
		return 0, io.EOF
	}
	return f.reader.Read(p)
}

func (f *ReaderFile) Close() error {
	return f.reader.Close()
}

func (f *ReaderFile) Stat() (fi os.FileInfo, err error) {
	return f.stat, nil
}

func (f *ReaderFile) Size() (int64, error) {
	if f.stat == nil {
		return 0, errors.New("File size unknown")
	}
	return f.stat.Size(), nil
}
