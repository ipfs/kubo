package atomicfile

import (
	"io"
	"os"
	"path/filepath"
)

// File represents an atomic file writer
type File struct {
	*os.File
	path string
}

// New creates a new atomic file writer
func New(path string, mode os.FileMode) (*File, error) {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path))
	if err != nil {
		return nil, err
	}

	if err := tempFile.Chmod(mode); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, err
	}

	return &File{
		File: tempFile,
		path: path,
	}, nil
}

// Close atomically replaces the target file with the temporary file
func (f *File) Close() error {
	if err := f.File.Close(); err != nil {
		os.Remove(f.File.Name())
		return err
	}

	if err := os.Rename(f.File.Name(), f.path); err != nil {
		os.Remove(f.File.Name())
		return err
	}

	return nil
}

// Abort removes the temporary file without replacing the target
func (f *File) Abort() error {
	f.File.Close()
	return os.Remove(f.File.Name())
}

// ReadFrom reads from the given reader into the atomic file
func (f *File) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(f.File, r)
}
