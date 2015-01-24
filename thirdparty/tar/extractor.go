package tar

import (
	"archive/tar"
	"io"
	"os"
	fp "path/filepath"
	"strings"
)

type Extractor struct {
	Path string
}

func (te *Extractor) Extract(reader io.Reader) error {
	tarReader := tar.NewReader(reader)

	// Check if the output path already exists, so we know whether we should
	// create our output with that name, or if we should put the output inside
	// a preexisting directory
	exists := true
	pathIsDir := false
	if stat, err := os.Stat(te.Path); err != nil && os.IsNotExist(err) {
		exists = false
	} else if err != nil {
		return err
	} else if stat.IsDir() {
		pathIsDir = true
	}

	// files come recursively in order (i == 0 is root directory)
	for i := 0; ; i++ {
		header, err := tarReader.Next()
		if err != nil && err != io.EOF {
			return err
		}
		if header == nil || err == io.EOF {
			break
		}

		if header.Typeflag == tar.TypeDir {
			err = te.extractDir(header, i, exists)
			if err != nil {
				return err
			}
			continue
		}

		err = te.extractFile(header, tarReader, i, exists, pathIsDir)
		if err != nil {
			return err
		}
	}
	return nil
}

func (te *Extractor) extractDir(h *tar.Header, depth int, exists bool) error {
	pathElements := strings.Split(h.Name, "/")
	if !exists {
		pathElements = pathElements[1:]
	}
	path := fp.Join(pathElements...)
	path = fp.Join(te.Path, path)
	if depth == 0 {
		// if this is the root root directory, use it as the output path for remaining files
		te.Path = path
	}

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	return nil
}

func (te *Extractor) extractFile(h *tar.Header, r *tar.Reader, depth int, exists bool, pathIsDir bool) error {
	var path string
	if depth == 0 {
		// if depth is 0, this is the only file (we aren't 'ipfs get'ing a directory)
		switch {
		case exists && !pathIsDir:
			return os.ErrExist
		case exists && pathIsDir:
			path = fp.Join(te.Path, h.Name)
		case !exists:
			path = te.Path
		}
	} else {
		// we are outputting a directory, this file is inside of it
		pathElements := strings.Split(h.Name, "/")[1:]
		path = fp.Join(pathElements...)
		path = fp.Join(te.Path, path)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, r)
	if err != nil {
		return err
	}

	return nil
}
