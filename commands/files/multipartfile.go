package files

import (
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	multipartFormdataType = "multipart/form-data"
	multipartMixedType    = "multipart/mixed"

	contentTypeHeader = "Content-Type"
)

// MultipartFile implements File, and is created from a `multipart.Part`.
// It can be either a directory or file (checked by calling `IsDirectory()`).
type MultipartFile struct {
	File

	Part      *multipart.Part
	Reader    *multipart.Reader
	Mediatype string
	stat      os.FileInfo
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

	name := f.FileName()
	fileInfoString := part.Header.Get("File-Info")
	if fileInfoString == "" {
		if f.IsDirectory() {
			f.stat = NewDummyDirectoryFileInfo(name)
		} else {
			f.stat = NewDummyRegularFileInfo(name)
		}
	} else {
		version, params, err := mime.ParseMediaType(fileInfoString)
		if version != "ipfs/v1" {
			return nil, fmt.Errorf(
				"unrecognized File-Info version: %s (%s)", version, fileInfoString)
		}
		sizeString, ok := params["size"]
		if !ok {
			return nil, fmt.Errorf(
				"File-Info missing \"size\" parameter: %s", fileInfoString)
		}
		size, err := strconv.ParseInt(sizeString, 0, 64)
		if err != nil {
			return nil, err
		}
		modeString, ok := params["mode"]
		if !ok {
			return nil, fmt.Errorf(
				"File-Info missing \"mode\" parameter: %s", fileInfoString)
		}
		mode, err := strconv.ParseUint(modeString, 0, 32)
		if err != nil {
			return nil, err
		}
		modTimeString, ok := params["mod-time"]
		if !ok {
			return nil, fmt.Errorf(
				"File-Info missing \"mod-time\" parameter: %s", fileInfoString)
		}
		modTime, err := time.Parse(time.RFC3339Nano, modTimeString)
		if err != nil {
			return nil, err
		}
		f.stat = &fileInfo{
			name:    name,
			size:    size,
			mode:    os.FileMode(mode),
			modTime: modTime,
		}
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
	filename, err := url.QueryUnescape(f.Part.FileName())
	if err != nil {
		// if there is a unescape error, just treat the name as unescaped
		return f.Part.FileName()
	}
	return filename
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

func (f *MultipartFile) Stat() (fi os.FileInfo, err error) {
	return f.stat, nil
}
