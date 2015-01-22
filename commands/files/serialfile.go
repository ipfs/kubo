package files

import (
	"io"
	"os"
	fp "path"
	"sort"
	"syscall"
)

type sortFIByName []string

func (es sortFIByName) Len() int           { return len(es) }
func (es sortFIByName) Swap(i, j int)      { es[i], es[j] = es[j], es[i] }
func (es sortFIByName) Less(i, j int) bool { return es[i] < es[j] }

// serialFile implements File, and reads from a path on the OS filesystem.
// No more than one file will be opened at a time (directories will advance
// to the next file when NextFile() is called).
type serialFile struct {
	path    string
	files   []string
	current *os.File
}

func NewSerialFile(path string, file *os.File) (File, error) {
	// try to list the content of the directory, and if it fails with no entries
	// found, assume it is not a directory and return a ReaderFile. if we can,
	// we should try to test that the error is ENOENT or ENOTDIR.
	// See getdents(2)
	contents, err := file.Readdirnames(0)
	if err != nil && len(contents) == 0 {
		return &ReaderFile{path, file}, nil
	} else if err != nil {
		return nil, err
	}

	// we no longer need our root directory file (we already statted the contents),
	// so close it
	err = file.Close()
	if err != nil {
		return nil, err
	}

	// make sure contents are sorted so -- repeatably -- we get the same inputs.
	sort.Sort(sortFIByName(contents))

	return &serialFile{path, contents, nil}, nil
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

	// open the next file
	filePath := fp.Join(f.path, f.files[0])
	f.files = f.files[1:]

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	f.current = file

	// recursively call the constructor on the next file
	// if it's a regular file, we will open it as a ReaderFile
	// if it's a directory, files in it will be opened serially
	return NewSerialFile(filePath, file)
}

func (f *serialFile) FileName() string {
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
