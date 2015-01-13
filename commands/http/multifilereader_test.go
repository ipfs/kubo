package http

import (
	"io"
	"mime/multipart"
	"strings"
	"testing"

	files "github.com/jbenet/go-ipfs/commands/files"
)

func TestOutput(t *testing.T) {
	text := "Some text! :)"
	fileset := []files.File{
		&files.ReaderFile{"file.txt", strings.NewReader(text)},
		&files.SliceFile{"boop", []files.File{
			&files.ReaderFile{"boop/a.txt", strings.NewReader("bleep")},
			&files.ReaderFile{"boop/b.txt", strings.NewReader("bloop")},
		}},
		&files.ReaderFile{"beep.txt", strings.NewReader("beep")},
	}
	sf := &files.SliceFile{"", fileset}
	buf := make([]byte, 20)

	// testing output by reading it with the go stdlib "mime/multipart" Reader
	mfr := NewMultiFileReader(sf, true)
	mpReader := multipart.NewReader(mfr, mfr.Boundary())

	part, err := mpReader.NextPart()
	if part == nil || err != nil {
		t.Error("Expected non-nil part, nil error")
	}
	mpf, err := files.NewFileFromPart(part)
	if mpf == nil || err != nil {
		t.Error("Expected non-nil MultipartFile, nil error")
	}
	if mpf.IsDirectory() {
		t.Error("Expected file to not be a directory")
	}
	if mpf.FileName() != "file.txt" {
		t.Error("Expected filename to be \"file.txt\"")
	}
	if n, err := mpf.Read(buf); n != len(text) || err != nil {
		t.Error("Expected to read from file", n, err)
	}
	if string(buf[:len(text)]) != text {
		t.Error("Data read was different than expected")
	}

	part, err = mpReader.NextPart()
	if part == nil || err != nil {
		t.Error("Expected non-nil part, nil error")
	}
	mpf, err = files.NewFileFromPart(part)
	if mpf == nil || err != nil {
		t.Error("Expected non-nil MultipartFile, nil error")
	}
	if !mpf.IsDirectory() {
		t.Error("Expected file to be a directory")
	}
	if mpf.FileName() != "boop" {
		t.Error("Expected filename to be \"boop\"")
	}

	child, err := mpf.NextFile()
	if child == nil || err != nil {
		t.Error("Expected to be able to read a child file")
	}
	if child.IsDirectory() {
		t.Error("Expected file to not be a directory")
	}
	if child.FileName() != "boop/a.txt" {
		t.Error("Expected filename to be \"some/file/path\"")
	}

	child, err = mpf.NextFile()
	if child == nil || err != nil {
		t.Error("Expected to be able to read a child file")
	}
	if child.IsDirectory() {
		t.Error("Expected file to not be a directory")
	}
	if child.FileName() != "boop/b.txt" {
		t.Error("Expected filename to be \"some/file/path\"")
	}

	child, err = mpf.NextFile()
	if child != nil || err != io.EOF {
		t.Error("Expected to get (nil, io.EOF)")
	}

	part, err = mpReader.NextPart()
	if part == nil || err != nil {
		t.Error("Expected non-nil part, nil error")
	}
	mpf, err = files.NewFileFromPart(part)
	if mpf == nil || err != nil {
		t.Error("Expected non-nil MultipartFile, nil error")
	}

	part, err = mpReader.NextPart()
	if part != nil || err != io.EOF {
		t.Error("Expected to get (nil, io.EOF)")
	}
}
