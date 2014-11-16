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

type File interface {
	io.ReadCloser
	FileName() string
	IsDirectory() bool
	NextFile() (File, error)
}

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
