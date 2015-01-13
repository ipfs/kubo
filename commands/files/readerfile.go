package files

import "io"

// ReaderFile is a implementation of File created from an `io.Reader`.
// ReaderFiles are never directories, and can be read from and closed.
type ReaderFile struct {
	Filename string
	Reader   io.Reader
}

func (f *ReaderFile) IsDirectory() bool {
	return false
}

func (f *ReaderFile) NextFile() (File, error) {
	return nil, ErrNotDirectory
}

func (f *ReaderFile) FileName() string {
	return f.Filename
}

func (f *ReaderFile) Read(p []byte) (int, error) {
	return f.Reader.Read(p)
}

func (f *ReaderFile) Close() error {
	closer, ok := f.Reader.(io.Closer)
	if !ok {
		return nil
	}
	return closer.Close()
}
