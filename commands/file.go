package commands

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
)

const (
	multipartFormdataType = "multipart/form-data"
	multipartMixedType    = "multipart/mixed"

	contentTypeHeader = "Content-Type"
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

// MultipartFile implements File, and is created from a `multipart.Part`.
// It can be either a directory or file (checked by calling `IsDirectory()`).
type MultipartFile struct {
	File

	Part      *multipart.Part
	Reader    *multipart.Reader
	Mediatype string
}

func NewFileFromPart(part *multipart.Part) (File, error) {
	f := &MultipartFile{
		Part: part,
	}

	contentType := part.Header.Get(contentTypeHeader)

	var params map[string]string
	var err error
	f.Mediatype, params, err = mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}

	if f.IsDirectory() {
		boundary, found := params["boundary"]
		if !found {
			return nil, http.ErrMissingBoundary
		}

		f.Reader = multipart.NewReader(part, boundary)
	}

	return f, nil
}

func (f *MultipartFile) IsDirectory() bool {
	return f.Mediatype == multipartFormdataType || f.Mediatype == multipartMixedType
}

func (f *MultipartFile) NextFile() (File, error) {
	if !f.IsDirectory() {
		return nil, ErrNotDirectory
	}

	part, err := f.Reader.NextPart()
	if err != nil {
		return nil, err
	}

	return NewFileFromPart(part)
}

func (f *MultipartFile) FileName() string {
	return f.Part.FileName()
}

func (f *MultipartFile) Read(p []byte) (int, error) {
	if f.IsDirectory() {
		return 0, ErrNotReader
	}
	return f.Part.Read(p)
}

func (f *MultipartFile) Close() error {
	if f.IsDirectory() {
		return ErrNotReader
	}
	return f.Part.Close()
}

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
