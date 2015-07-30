package corehttp

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/ipfs/go-ipfs/commands/files"
	"github.com/ipfs/go-ipfs/importer"
	chunk "github.com/ipfs/go-ipfs/importer/chunk"
	dag "github.com/ipfs/go-ipfs/merkledag"
	ft "github.com/ipfs/go-ipfs/unixfs"
)

// helper functions for the gateway handler

func (i *gatewayHandler) newDagFromReader(r io.Reader) (*dag.Node, error) {
	// TODO(cryptix): change and remove this helper once PR1136 is merged
	// return ufs.AddFromReader(i.node, r.Body)
	return importer.BuildDagFromReader(
		r, i.node.DAG, chunk.DefaultSplitter, importer.BasicPinnerCB(i.node.Pinning.GetManual()))
}

// add file, or directory recursively
func (i *gatewayHandler) addFileRecursive(file files.File) (*dag.Node, error) {
	if file.IsDirectory() {
		return i.addDirRecursive(file)
	}
	return i.newDagFromReader(file)
}

// add a directory recursively
func (i *gatewayHandler) addDirRecursive(file files.File) (*dag.Node, error) {
	tree := &dag.Node{Data: ft.FolderPBData()}
	// log.Infof("adding directory: %s", name)

	for {
		file, err := file.NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if file == nil {
			break
		}

		node, err := i.addFileRecursive(file)
		if err != nil {
			return nil, err
		}

		if node != nil {
			_, name := path.Split(file.FileName())

			err = tree.AddNodeLink(name, node)
			if err != nil {
				return nil, err
			}
		}
	}

	_, err := i.node.DAG.Add(tree)
	if err != nil {
		return nil, err
	}

	// TODO: pinning?
	//i.node.Pinning.GetManual().PinWithMode(k, pin.Indirect)
	return tree, nil
}

// gzip decompress body into byte slice
func decompressBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	gr, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	buf, err := ioutil.ReadAll(gr)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// unarchive decompressed tar ball into temp dir
func unarchiveTarBall(archive []byte) (string, error) {
	tmpDir, err := ioutil.TempDir("", "ipfs")
	if err != nil {
		return "", err
	}

	tr := tar.NewReader(bytes.NewBuffer(archive))
	var rootDir string // should be the first file in the tarball
	// first run through the loop is the top level directory name
	first := true
	for {
		fileHeader, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		// determine based on name if this is a dir or a file
		if _, f := path.Split(fileHeader.Name); f == "" {
			if err := os.Mkdir(path.Join(tmpDir, fileHeader.Name), 0700); err != nil {
				return "", err
			}
			if first {
				rootDir = fileHeader.Name
			}
		} else {
			if first {
				return "", fmt.Errorf("First entry in tarball must be directory")
			}
			// read in the next file in the archive
			fileBytes, err := ioutil.ReadAll(tr)
			if err != nil {
				return "", err
			}
			if err := ioutil.WriteFile(path.Join(tmpDir, fileHeader.Name), fileBytes, os.FileMode(fileHeader.Mode)); err != nil {
				return "", err
			}
		}
		first = false
	}
	return path.Join(tmpDir, rootDir), nil
}
