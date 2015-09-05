package files

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	fp "path/filepath"
	"syscall"
)

// serialFile implements File, and reads from a path on the OS filesystem.
// No more than one file will be opened at a time (directories will advance
// to the next file when NextFile() is called).
type serialFile struct {
	name    string
	path    string
	files   []os.FileInfo
	stat    os.FileInfo
	current File

	next    chan File
	openErr chan error
}

func NewSerialFile(name, path string, stat os.FileInfo) (File, error) {
	switch mode := stat.Mode(); {
	case mode.IsRegular():
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return NewReaderFile(name, path, file, stat), nil
	case mode.IsDir():
		// for directories, stat all of the contents first, so we know what files to
		// open when NextFile() is called
		contents, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}
		sf := &serialFile{
			name:    name,
			path:    path,
			files:   contents,
			stat:    stat,
			next:    make(chan File, 1),
			openErr: make(chan error, 1),
		}
		go sf.asyncFileLoader()
		return sf, nil

	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		return NewLinkFile(name, path, target, stat), nil
	default:
		return nil, fmt.Errorf("Unrecognized file type for %s: %s", name, mode.String())
	}
}

func (f *serialFile) asyncFileLoader() {
	defer close(f.next)

	for _, stat := range f.files {
		fileName := fp.Join(f.name, stat.Name())
		filePath := fp.Join(f.path, stat.Name())

		// recursively call the constructor on the next file
		// if it's a regular file, we will open it as a ReaderFile
		// if it's a directory, files in it will be opened serially
		sf, err := NewSerialFile(fileName, filePath, stat)
		if err != nil {
			f.openErr <- err
			return
		}

		f.next <- sf
	}
}

func (f *serialFile) IsDirectory() bool {
	// non-directories get created as a ReaderFile, so serialFiles should only
	// represent directories
	return true
}

func (f *serialFile) NextFile() (File, error) {
	// if a file was opened previously, close it
	err := f.Close()
	if err != nil {
		return nil, err
	}

	select {
	case fi, ok := <-f.next:
		if !ok {
			return nil, io.EOF
		}
		f.current = fi
		return fi, nil

	case err := <-f.openErr:
		return nil, err
	}
}

func (f *serialFile) FileName() string {
	return f.name
}

func (f *serialFile) FullPath() string {
	return f.path
}

func (f *serialFile) Read(p []byte) (int, error) {
	return 0, ErrNotReader
}

func (f *serialFile) Close() error {
	// close the current file if there is one
	if f.current != nil {
		err := f.current.Close()
		// ignore EINVAL error, the file might have already been closed
		if err != nil && err != syscall.EINVAL {
			return err
		}
	}

	return nil
}

func (f *serialFile) Stat() os.FileInfo {
	return f.stat
}

func (f *serialFile) Size() (int64, error) {
	if !f.stat.IsDir() {
		return f.stat.Size(), nil
	}

	var du int64
	err := fp.Walk(f.FileName(), func(p string, fi os.FileInfo, err error) error {
		if fi != nil && fi.Mode()&(os.ModeSymlink|os.ModeNamedPipe) == 0 {
			du += fi.Size()
		}
		return nil
	})
	return du, err
}
