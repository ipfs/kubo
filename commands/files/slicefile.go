package files

import "io"

// SliceFile implements File, and provides simple directory handling.
// It contains children files, and is created from a `[]File`.
// SliceFiles are always directories, and can't be read from or closed.
type SliceFile struct {
	Filename string
	Files    []File
}

func (f *SliceFile) IsDirectory() bool {
	return true
}

func (f *SliceFile) NextFile() (File, error) {
	if len(f.Files) == 0 {
		return nil, io.EOF
	}
	file := f.Files[0]
	f.Files = f.Files[1:]
	return file, nil
}

func (f *SliceFile) FileName() string {
	return f.Filename
}

func (f *SliceFile) Read(p []byte) (int, error) {
	return 0, ErrNotReader
}

func (f *SliceFile) Close() error {
	return ErrNotReader
}
