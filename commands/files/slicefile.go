package files

import "io"

// SliceFile implements File, and provides simple directory handling.
// It contains children files, and is created from a `[]File`.
// SliceFiles are always directories, and can't be read from or closed.
type SliceFile struct {
	filename string
	files    []File
	n        int
}

func NewSliceFile(filename string, files []File) *SliceFile {
	return &SliceFile{filename, files, 0}
}

func (f *SliceFile) IsDirectory() bool {
	return true
}

func (f *SliceFile) NextFile() (File, error) {
	if f.n >= len(f.files) {
		return nil, io.EOF
	}
	file := f.files[f.n]
	f.n++
	return file, nil
}

func (f *SliceFile) FileName() string {
	return f.filename
}

func (f *SliceFile) Read(p []byte) (int, error) {
	return 0, ErrNotReader
}

func (f *SliceFile) Close() error {
	return ErrNotReader
}

func (f *SliceFile) Peek(n int) File {
	return f.files[n]
}

func (f *SliceFile) Length() int {
	return len(f.files)
}
