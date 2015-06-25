package files

import (
	"errors"
	"io"
	"os"
)

// SliceFile implements File, and provides simple directory handling.
// It contains children files, and is created from a `[]File`.
// SliceFiles are always directories, and can't be read from or closed.
type SliceFile struct {
	filename string
	files    []File
	stat     os.FileInfo
	n        int
}

func NewSliceFile(filename string, file *os.File, files []File) (f *SliceFile, err error) {
	var stat os.FileInfo
	if file == nil {
		stat = NewDummyDirectoryFileInfo(filename)
	} else {
		stat, err = file.Stat()
		if err != nil {
			return nil, err
		}
	}

	return &SliceFile{filename, files, stat, 0}, nil
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

func (f *SliceFile) Stat() (fi os.FileInfo, err error) {
	return f.stat, nil
}

func (f *SliceFile) Peek(n int) File {
	return f.files[n]
}

func (f *SliceFile) Length() int {
	return len(f.files)
}

func (f *SliceFile) Size() (int64, error) {
	var size int64

	for _, file := range f.files {
		sizeFile, ok := file.(SizeFile)
		if !ok {
			return 0, errors.New("Could not get size of child file")
		}

		s, err := sizeFile.Size()
		if err != nil {
			return 0, err
		}
		size += s
	}

	return size, nil
}
