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
	offset   int64
	baseInfo ExtraInfo
}

func NewReaderFile(filename, path string, reader io.ReadCloser, stat os.FileInfo) *ReaderFile {
	return &ReaderFile{filename, path, reader, stat, 0, PosInfo{0, path}}
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

func (f *ReaderFile) ExtraInfo() ExtraInfo {
	return f.baseInfo.Clone(f.offset)
}

func (f *ReaderFile) SetExtraInfo(info ExtraInfo) error {
	f.baseInfo = info
	return nil
}

func (f *ReaderFile) Read(p []byte) (int, error) {
	res, err := f.reader.Read(p)
	f.offset += int64(res)
	return res, err
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
