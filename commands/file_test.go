package commands

import (
	"io"
	"mime/multipart"
	"strings"
	"testing"
)

func TestSliceFiles(t *testing.T) {
	name := "testname"
	files := []File{
		&ReaderFile{"file.txt", strings.NewReader("Some text!\n")},
		&ReaderFile{"beep.txt", strings.NewReader("beep")},
		&ReaderFile{"boop.txt", strings.NewReader("boop")},
	}
	buf := make([]byte, 20)

	sf := &SliceFile{name, files}

	if !sf.IsDirectory() {
		t.Error("SliceFile should always be a directory")
	}
	if n, err := sf.Read(buf); n > 0 || err != ErrNotReader {
		t.Error("Shouldn't be able to call `Read` on a SliceFile")
	}
	if err := sf.Close(); err != ErrNotReader {
		t.Error("Shouldn't be able to call `Close` on a SliceFile")
	}

	file, err := sf.NextFile()
	if file == nil || err != nil {
		t.Error("Expected a file and nil error")
	}
	read, err := file.Read(buf)
	if read != 11 || err != nil {
		t.Error("NextFile got a file in the wrong order")
	}

	file, err = sf.NextFile()
	if file == nil || err != nil {
		t.Error("Expected a file and nil error")
	}
	file, err = sf.NextFile()
	if file == nil || err != nil {
		t.Error("Expected a file and nil error")
	}

	file, err = sf.NextFile()
	if file != nil || err != io.EOF {
		t.Error("Expected a nil file and io.EOF")
	}
}

func TestReaderFiles(t *testing.T) {
	message := "beep boop"
	rf := &ReaderFile{"file.txt", strings.NewReader(message)}
	buf := make([]byte, len(message))

	if rf.IsDirectory() {
		t.Error("ReaderFile should never be a directory")
	}
	file, err := rf.NextFile()
	if file != nil || err != ErrNotDirectory {
		t.Error("Expected a nil file and ErrNotDirectory")
	}

	if n, err := rf.Read(buf); n == 0 || err != nil {
		t.Error("Expected to be able to read")
	}
	if err := rf.Close(); err != nil {
		t.Error("Should be able to close")
	}
	if n, err := rf.Read(buf); n != 0 || err != io.EOF {
		t.Error("Expected EOF when reading after close")
	}
}

func TestMultipartFiles(t *testing.T) {
	data := `
--Boundary!
Content-Type: text/plain
Content-Disposition: file; filename="name"
Some-Header: beep

beep
--Boundary!
Content-Type: multipart/mixed; boundary=OtherBoundary
Content-Disposition: file; filename="dir"

--OtherBoundary
Content-Type: text/plain
Content-Disposition: file; filename="some/file/path"

test
--OtherBoundary
Content-Type: text/plain

boop
--OtherBoundary
Content-Type: text/plain

bloop
--OtherBoundary--
--Boundary!--

`

	reader := strings.NewReader(data)
	mpReader := multipart.NewReader(reader, "Boundary!")
	buf := make([]byte, 20)

	// test properties of a file created from the first part
	part, err := mpReader.NextPart()
	if part == nil || err != nil {
		t.Error("Expected non-nil part, nil error")
	}
	mpf, err := NewFileFromPart(part)
	if mpf == nil || err != nil {
		t.Error("Expected non-nil MultipartFile, nil error")
	}
	if mpf.IsDirectory() {
		t.Error("Expected file to not be a directory")
	}
	if mpf.FileName() != "name" {
		t.Error("Expected filename to be \"name\"")
	}
	if file, err := mpf.NextFile(); file != nil || err != ErrNotDirectory {
		t.Error("Expected a nil file and ErrNotDirectory")
	}
	if n, err := mpf.Read(buf); n != 4 || err != nil {
		t.Error("Expected to be able to read 4 bytes")
	}
	if err := mpf.Close(); err != nil {
		t.Error("Expected to be able to close file")
	}

	// test properties of file created from second part (directory)
	part, err = mpReader.NextPart()
	if part == nil || err != nil {
		t.Error("Expected non-nil part, nil error")
	}
	mpf, err = NewFileFromPart(part)
	if mpf == nil || err != nil {
		t.Error("Expected non-nil MultipartFile, nil error")
	}
	if !mpf.IsDirectory() {
		t.Error("Expected file to be a directory")
	}
	if mpf.FileName() != "dir" {
		t.Error("Expected filename to be \"dir\"")
	}
	if n, err := mpf.Read(buf); n > 0 || err != ErrNotReader {
		t.Error("Shouldn't be able to call `Read` on a directory")
	}
	if err := mpf.Close(); err != ErrNotReader {
		t.Error("Shouldn't be able to call `Close` on a directory")
	}

	// test properties of first child file
	child, err := mpf.NextFile()
	if child == nil || err != nil {
		t.Error("Expected to be able to read a child file")
	}
	if child.IsDirectory() {
		t.Error("Expected file to not be a directory")
	}
	if child.FileName() != "some/file/path" {
		t.Error("Expected filename to be \"some/file/path\"")
	}

	// test processing files out of order
	child, err = mpf.NextFile()
	if child == nil || err != nil {
		t.Error("Expected to be able to read a child file")
	}
	child2, err := mpf.NextFile()
	if child == nil || err != nil {
		t.Error("Expected to be able to read a child file")
	}
	if n, err := child2.Read(buf); n != 5 || err != nil {
		t.Error("Expected to be able to read")
	}
	if n, err := child.Read(buf); n != 0 || err == nil {
		t.Error("Expected to not be able to read after advancing NextFile() past this file")
	}

	// make sure the end is handled properly
	child, err = mpf.NextFile()
	if child != nil || err == nil {
		t.Error("Expected NextFile to return (nil, EOF)")
	}
}
