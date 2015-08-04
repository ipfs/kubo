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
	rootExists := true
	rootIsDir := false
	if stat, err := os.Stat(te.Path); err != nil && os.IsNotExist(err) {
		rootExists = false
	} else if err != nil {
		return err
	} else if stat.IsDir() {
		rootIsDir = true
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
			if err := te.extractDir(header, i, rootExists); err != nil {
				return err
			}
			continue
		}

		if err := te.extractFile(header, tarReader, i, rootExists, rootIsDir); err != nil {
			return err
		}
	}
	return nil
}

func (te *Extractor) extractDir(h *tar.Header, depth int, rootExists bool) error {
	pathElements := strings.Split(h.Name, "/")
	if !rootExists {
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

func (te *Extractor) extractFile(h *tar.Header, r *tar.Reader, depth int, rootExists bool, rootIsDir bool) error {
	path := te.Path
	if depth == 0 && rootExists {
		// if depth is 0, this is the only file (we aren't 'ipfs get'ing a directory)
		if rootIsDir { // putting file inside of a root dir.
			path = fp.Join(te.Path, h.Name)
		}
		// else if the file exists, just overwrite it.
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
