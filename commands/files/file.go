package files

import (
	"errors"
	"io"
)

var (
	ErrNotDirectory = errors.New("Couln't call NextFile(), this isn't a directory")
	ErrNotReader    = errors.New("This file is a directory, can't use Reader functions")
)

// File is an interface that provides functionality for handling files/directories
// as values that can be supplied to commands. For directories, child files are
// accessed serially by calling `NextFile()`.
type File interface {
	// Files implement ReadCloser, but can only be read from or closed if they are not directories
	io.ReadCloser

	// FileName returns a full filename path associated with this file
	FileName() string

	// IsDirectory returns true if the File is a directory (and therefore supports calling `NextFile`)
	// and false if the File is a normal file (and therefor supports calling `Read` and `Close`)
	IsDirectory() bool

	// NextFile returns the next child file available (if the File is a directory).
	// It will return (nil, io.EOF) if no more files are available.
	// If the file is a regular file (not a directory), NextFile will return a non-nil error.
	NextFile() (File, error)
}
