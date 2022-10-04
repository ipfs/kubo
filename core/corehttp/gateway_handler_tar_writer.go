package corehttp

import (
	"archive/tar"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	files "github.com/ipfs/go-ipfs-files"
)

type TarWriter struct {
	TarW *tar.Writer
	root string
}

// NewTarWriter wraps given io.Writer into a new tar writer
func NewTarWriter(w io.Writer) (*TarWriter, error) {
	return &TarWriter{
		TarW: tar.NewWriter(w),
	}, nil
}

func (w *TarWriter) writeDir(f files.Directory, fpath string) error {
	if err := writeDirHeader(w.TarW, fpath); err != nil {
		return err
	}

	it := f.Entries()
	for it.Next() {
		if err := w.WriteFile(it.Node(), path.Join(fpath, it.Name())); err != nil {
			return err
		}
	}
	return it.Err()
}

func (w *TarWriter) writeFile(f files.File, fpath string) error {
	size, err := f.Size()
	if err != nil {
		return err
	}

	if err := writeFileHeader(w.TarW, fpath, uint64(size)); err != nil {
		return err
	}

	if _, err := io.Copy(w.TarW, f); err != nil {
		return err
	}
	w.TarW.Flush()
	return nil
}

// WriteNode adds a node to the archive.
func (w *TarWriter) WriteFile(nd files.Node, fpath string) error {
	if w.root == "" {
		w.root = fpath
	}

	if !strings.HasPrefix(fpath, w.root) {
		fpath = strings.Replace(fpath, ".", "", -1)
		fpath = strings.Replace(fpath, "..", "", -1)
		fpath = path.Join(w.root, fpath)
	}

	switch nd := nd.(type) {
	case *files.Symlink:
		return writeSymlinkHeader(w.TarW, nd.Target, fpath)
	case files.File:
		return w.writeFile(nd, fpath)
	case files.Directory:
		return w.writeDir(nd, fpath)
	default:
		return fmt.Errorf("file type %T is not supported", nd)
	}
}

// Close closes the tar writer.
func (w *TarWriter) Close() error {
	return w.TarW.Close()
}

func writeDirHeader(w *tar.Writer, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Typeflag: tar.TypeDir,
		Mode:     0777,
		ModTime:  time.Now().Truncate(time.Second),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeFileHeader(w *tar.Writer, fpath string, size uint64) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Size:     int64(size),
		Typeflag: tar.TypeReg,
		Mode:     0644,
		ModTime:  time.Now().Truncate(time.Second),
		// TODO: set mode, dates, etc. when added to unixFS
	})
}

func writeSymlinkHeader(w *tar.Writer, target, fpath string) error {
	return w.WriteHeader(&tar.Header{
		Name:     fpath,
		Linkname: target,
		Mode:     0777,
		Typeflag: tar.TypeSymlink,
	})
}
