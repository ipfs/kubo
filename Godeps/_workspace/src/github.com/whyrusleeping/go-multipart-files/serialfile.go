package files

import (
	"io"
	"os"
	fp "path/filepath"
	"sort"
	"syscall"
)

type sortFIByName []os.FileInfo

func (es sortFIByName) Len() int           { return len(es) }
func (es sortFIByName) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es sortFIByName) Less(i, j int) bool { return es[i].Name() < es[j].Name() }

// serialFile implements File, and reads from a path on the OS filesystem.
// No more than one file will be opened at a time (directories will advance
// to the next file when NextFile() is called).
type serialFile struct {
	name    string
	path    string
	files   []os.FileInfo
	stat    os.FileInfo
	current *os.File
}

func NewSerialFile(name, path string, stat os.FileInfo) (File, error) {
	if stat.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}

		return NewLinkFile("", path, target, stat), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return newSerialFile(name, path, file, stat)
}

func newSerialFile(name, path string, file *os.File, stat os.FileInfo) (File, error) {
	// for non-directories, return a ReaderFile
	if !stat.IsDir() {
		return &ReaderFile{name, path, file, stat}, nil
	}

	// for directories, stat all of the contents first, so we know what files to
	// open when NextFile() is called
	contents, err := file.Readdir(0)
	if err != nil {
		return nil, err
	}

	// we no longer need our root directory file (we already statted the contents),
	// so close it
	if err := file.Close(); err != nil {
		return nil, err
	}

	// make sure contents are sorted so -- repeatably -- we get the same inputs.
	sort.Sort(sortFIByName(contents))

	return &serialFile{name, path, contents, stat, nil}, nil
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

	// if there aren't any files left in the root directory, we're done
	if len(f.files) == 0 {
		return nil, io.EOF
	}

	stat := f.files[0]
	f.files = f.files[1:]

	// open the next file
	fileName := fp.Join(f.name, stat.Name())
	filePath := fp.Join(f.path, stat.Name())
	st, err := os.Lstat(filePath)
	if err != nil {
		return nil, err
	}

	if st.Mode()&os.ModeSymlink != 0 {
		f.current = nil
		target, err := os.Readlink(filePath)
		if err != nil {
			return nil, err
		}
		return NewLinkFile(fileName, filePath, target, st), nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	f.current = file

	// recursively call the constructor on the next file
	// if it's a regular file, we will open it as a ReaderFile
	// if it's a directory, files in it will be opened serially
	return newSerialFile(fileName, filePath, file, stat)
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
