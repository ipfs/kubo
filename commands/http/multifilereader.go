package http

import (
	"bytes"
	"io"
	"mime/multipart"

	cmds "github.com/jbenet/go-ipfs/commands"
)

type MultiFileReader struct {
	io.Reader

	files       cmds.File
	currentFile cmds.File
	buf         bytes.Buffer
	mpWriter    *multipart.Writer
	closed      bool
}

func NewMultiFileReader(file cmds.File) *MultiFileReader {
	mfr := &MultiFileReader{
		files: file,
	}
	mfr.mpWriter = multipart.NewWriter(&mfr.buf)

	return mfr
}

func (mfr *MultiFileReader) Read(buf []byte) (written int, err error) {
	// if we are closed, end reading
	if mfr.closed && mfr.buf.Len() == 0 {
		return 0, io.EOF
	}

	// if the current file isn't set, advance to the next file
	if mfr.currentFile == nil {
		mfr.currentFile, err = mfr.files.NextFile()
		if err == io.EOF || (err == nil && mfr.currentFile == nil) {
			mfr.mpWriter.Close()
			mfr.closed = true
		} else if err != nil {
			return 0, err
		}

		if !mfr.closed {
			_, err := mfr.mpWriter.CreateFormFile("file", mfr.currentFile.FileName())
			if err != nil {
				return 0, err
			}
		}
	}

	var reader io.Reader

	if mfr.buf.Len() > 0 {
		// if the buffer has something in it, read from it
		reader = &mfr.buf

	} else if mfr.currentFile != nil {
		// otherwise, read from file data
		reader = mfr.currentFile
	}

	written, err = reader.Read(buf)
	if err == io.EOF && reader == mfr.currentFile {
		mfr.currentFile = nil
		return mfr.Read(buf)
	}
	return written, err
}

func (mfr *MultiFileReader) Boundary() string {
	return mfr.mpWriter.Boundary()
}
