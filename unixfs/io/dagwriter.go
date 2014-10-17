package io

import (
	"bytes"
	"io"

	"github.com/jbenet/go-ipfs/importer/chunk"
	dag "github.com/jbenet/go-ipfs/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("dagwriter")

type DagWriter struct {
	buf   bytes.Buffer
	first []byte
	mbf   ft.MultiBlock
	root  dag.Node

	dagserv  dag.DAGService
	splitter chunk.BlockSplitter

	node   *dag.Node
	seterr error
}

func NewDagWriter(ds dag.DAGService, splitter chunk.BlockSplitter) *DagWriter {
	dw := &DagWriter{
		dagserv:  ds,
		splitter: splitter,
	}
	dw.splitter.Push(&dw.buf)
	return dw
}

func (dw *DagWriter) processBlock(blk []byte) (err error) {
	// Store the block size in the root node
	dw.mbf.AddBlockSize(uint64(len(blk)))
	node := &dag.Node{Data: ft.WrapData(blk)}
	_, err = dw.dagserv.Add(node)
	if err != nil {
		dw.seterr = err
		log.Critical("Got error adding created node to dagservice: %s", err)
		return
	}

	// Add a link to this node without storing a reference to the memory
	err = dw.root.AddNodeLinkClean("", node)
	if err != nil {
		dw.seterr = err
		log.Critical("Got error adding created node to root node: %s", err)
	}
	return
}

func (dw *DagWriter) next() error {
	var err error
	if dw.first == nil {
		dw.first, err = dw.splitter.Next()
		return err
	}

	// splitter should not return an error when using dw.buf
	blk, _ := dw.splitter.Next()
	return dw.processBlock(blk)
}

func (dw *DagWriter) Write(b []byte) (n int, err error) {
	if dw.seterr != nil {
		return 0, dw.seterr
	}

	var N, max int
	for len(b) != 0 {
		max = dw.splitter.Size()
		if len(b) > max {
			N, err = dw.buf.Write(b[:max])
		} else {
			N, err = dw.buf.Write(b)
		}
		b = b[N:]
		n += N

		if dw.buf.Len() >= max {
			err = dw.next()
			if err != nil {
				return
			}
		}
	}
	return
}

// ReadFrom reads data from r until EOF or error.
// The return value n is the number of bytes read.
// Any error except io.EOF encountered during the
// read is also returned.
//
// The io.Copy function uses ReaderFrom if available.
func (dw *DagWriter) ReadFrom(r io.Reader) (n int64, err error) {
	// flush out buffer
	for dw.buf.Len() != 0 {
		err = dw.next()
		if err != nil {
			return
		}
	}

	dw.splitter.Push(r)

	if dw.first == nil {
		dw.first, err = dw.splitter.Next()
		n += int64(len(dw.first))
		if err != nil {
			if err == io.EOF {
				return n, nil
			}
			return
		}
	}
	var blk []byte
	for {
		blk, err = dw.splitter.Next()
		n += int64(len(blk))
		if err == nil {
			err = dw.processBlock(blk)
		}

		if err != nil {
			if err == io.EOF {
				return n, nil
			}
			return
		}
	}
}

// Flush the splitter and generate a dag.Node.
func (dw *DagWriter) Close() (err error) {
	var blk []byte
	for {
		blk, err = dw.splitter.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return err
		}

		err = dw.processBlock(blk)
		if err != nil {
			return err
		}
	}

	// Generate the root node data
	dw.mbf.Data = dw.first
	data, err := dw.mbf.GetBytes()
	if err != nil {
		dw.seterr = err
		log.Critical("Failed generating bytes for multiblock file: %s", err)
		return err
	}
	dw.root.Data = data

	// Add root node to the dagservice
	_, err = dw.dagserv.Add(&dw.root)
	if err != nil {
		dw.seterr = err
		log.Critical("Got error adding created node to dagservice: %s", err)
		return err
	}
	dw.node = &dw.root
	return nil
}

func (dw *DagWriter) GetNode() *dag.Node {
	return dw.node
}
