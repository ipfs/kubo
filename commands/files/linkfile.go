package files

import (
	"io"
	"os"
	"strings"
)

type Symlink struct {
	name   string
	path   string
	Target string
	stat   os.FileInfo
	root   bool

	reader io.Reader
}

func NewLinkFile(name, path, target string, stat os.FileInfo) *Symlink {
	return &Symlink{
		name:   name,
		path:   path,
		Target: target,
		stat:   stat,
		reader: strings.NewReader(target),
	}
}

func (lf *Symlink) IsDirectory() bool {
	return false
}

func (lf *Symlink) NextFile() (File, error) {
	return nil, io.EOF
}

func (f *Symlink) FileName() string {
	return f.name
}

func (f *Symlink) Close() error {
	return nil
}

func (f *Symlink) FullPath() string {
	return f.path
}

func (f *Symlink) Read(b []byte) (int, error) {
	return f.reader.Read(b)
}

func (f *Symlink) IsRoot() bool {
	return f.root
}
