package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DirWritable checks if a directory is writable. If the directory does
// not exist it is created with writable permission.
func DirWritable(dir string) error {
	if dir == "" {
		return errors.New("directory not specified")
	}

	var err error
	dir, err = ExpandHome(dir)
	if err != nil {
		return err
	}

	fi, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Directory does not exist, so create it.
			err = os.Mkdir(dir, 0775)
			if err == nil {
				return nil
			}
		}
		if errors.Is(err, fs.ErrPermission) {
			err = fs.ErrPermission
		}
		return fmt.Errorf("directory not writable: %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	// Directory exists, check that a file can be written.
	file, err := os.CreateTemp(dir, "writetest")
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			err = fs.ErrPermission
		}
		return fmt.Errorf("directory not writable: %s: %w", dir, err)
	}
	file.Close()
	return os.Remove(file.Name())
}

// ExpandHome expands the path to include the home directory if the path is
// prefixed with `~`. If it isn't prefixed with `~`, the path is returned
// as-is.
func ExpandHome(path string) (string, error) {
	if path == "" {
		return path, nil
	}

	if path[0] != '~' {
		return path, nil
	}

	if len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		return "", errors.New("cannot expand user-specific home dir")
	}

	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, path[1:]), nil
}

// FileExists return true if the file exists
func FileExists(filename string) bool {
	_, err := os.Lstat(filename)
	return !errors.Is(err, os.ErrNotExist)
}
