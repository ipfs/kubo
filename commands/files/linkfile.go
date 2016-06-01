package files

import (
	"io"
	"os"
	"strings"
)

type Symlink struct {
	name    string
	path    string
	abspath string
	Target  string
	stat    os.FileInfo

	reader io.Reader
}

func NewLinkFile(name, path, abspath, target string, stat os.FileInfo) File {
	return &Symlink{
		name:    name,
		path:    path,
		abspath: abspath,
		Target:  target,
		stat:    stat,
		reader:  strings.NewReader(target),
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
