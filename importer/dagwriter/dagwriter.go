package dagwriter

import (
	imp "github.com/jbenet/go-ipfs/importer"
	ft "github.com/jbenet/go-ipfs/importer/format"
	dag "github.com/jbenet/go-ipfs/merkledag"
	"github.com/jbenet/go-ipfs/util"
)

var log = util.Logger("dagwriter")

type DagWriter struct {
	dagserv   *dag.DAGService
	node      *dag.Node
	totalSize int64
	splChan   chan []byte
	done      chan struct{}
	splitter  imp.BlockSplitter
	seterr    error
}

func NewDagWriter(ds *dag.DAGService, splitter imp.BlockSplitter) *DagWriter {
	dw := new(DagWriter)
	dw.dagserv = ds
	dw.splChan = make(chan []byte, 8)
	dw.splitter = splitter
	dw.done = make(chan struct{})
	go dw.startSplitter()
	return dw
}

func (dw *DagWriter) startSplitter() {
	r := util.NewByteChanReader(dw.splChan)
	blkchan := dw.splitter.Split(r)
	first := <-blkchan
	mbf := new(ft.MultiBlock)
	root := new(dag.Node)

	for blkData := range blkchan {
		mbf.AddBlockSize(uint64(len(blkData)))
		node := &dag.Node{Data: ft.WrapData(blkData)}
		_, err := dw.dagserv.Add(node)
		if err != nil {
			dw.seterr = err
			log.Critical("Got error adding created node to dagservice: %s", err)
			return
		}
		err = root.AddNodeLinkClean("", node)
		if err != nil {
			dw.seterr = err
			log.Critical("Got error adding created node to root node: %s", err)
			return
		}
	}
	mbf.Data = first
	data, err := mbf.GetBytes()
	if err != nil {
		dw.seterr = err
		log.Critical("Failed generating bytes for multiblock file: %s", err)
		return
	}
	root.Data = data

	_, err = dw.dagserv.Add(root)
	if err != nil {
		dw.seterr = err
		log.Critical("Got error adding created node to dagservice: %s", err)
		return
	}
	dw.node = root
	dw.done <- struct{}{}
}

func (dw *DagWriter) Write(b []byte) (int, error) {
	if dw.seterr != nil {
		return 0, dw.seterr
	}
	dw.splChan <- b
	return len(b), nil
}

func (dw *DagWriter) Close() error {
	close(dw.splChan)
	<-dw.done
	return nil
}

func (dw *DagWriter) GetNode() *dag.Node {
	return dw.node
}
