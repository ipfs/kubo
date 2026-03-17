package atomicfile

import (
	"fmt"
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
	closeErr := f.File.Close()
	if closeErr != nil {
		// Try to cleanup temp file, but prioritize close error
		_ = os.Remove(f.File.Name())
		return closeErr
	}
	return os.Rename(f.File.Name(), f.path)
}

// Abort removes the temporary file without replacing the target
func (f *File) Abort() error {
	closeErr := f.File.Close()
	removeErr := os.Remove(f.File.Name())

	if closeErr != nil && removeErr != nil {
		return fmt.Errorf("abort failed: close: %w, remove: %v", closeErr, removeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	return removeErr
}

// ReadFrom reads from the given reader into the atomic file
func (f *File) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(f.File, r)
}
