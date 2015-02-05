package io

import (
	"bytes"
	"errors"

	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"

	chunk "github.com/jbenet/go-ipfs/unixfs/importer/chunk"
	mdag "github.com/jbenet/go-ipfs/struct/merkledag"
	ft "github.com/jbenet/go-ipfs/unixfs"
	ftpb "github.com/jbenet/go-ipfs/unixfs/pb"
	u "github.com/jbenet/go-ipfs/util"
)

var log = u.Logger("dagio")

// DagModifier is the only struct licensed and able to correctly
// perform surgery on a DAG 'file'
// Dear god, please rename this to something more pleasant
type DagModifier struct {
	dagserv mdag.DAGService
	curNode *mdag.Node

	pbdata   *ftpb.Data
	splitter chunk.BlockSplitter
}

func NewDagModifier(from *mdag.Node, serv mdag.DAGService, spl chunk.BlockSplitter) (*DagModifier, error) {
	pbd, err := ft.FromBytes(from.Data)
	if err != nil {
		return nil, err
	}

	return &DagModifier{
		curNode:  from.Copy(),
		dagserv:  serv,
		pbdata:   pbd,
		splitter: spl,
	}, nil
}

// WriteAt will modify a dag file in place
// NOTE: it currently assumes only a single level of indirection
func (dm *DagModifier) WriteAt(b []byte, offset uint64) (int, error) {

	// Check bounds
	if dm.pbdata.GetFilesize() < offset {
		return 0, errors.New("Attempted to perform write starting past end of file")
	}

	// First need to find where we are writing at
	end := uint64(len(b)) + offset

	// This shouldnt be necessary if we do subblocks sizes properly
	newsize := dm.pbdata.GetFilesize()
	if end > dm.pbdata.GetFilesize() {
		newsize = end
	}
	zeroblocklen := uint64(len(dm.pbdata.Data))
	origlen := len(b)

	if end <= zeroblocklen {
		log.Debug("Writing into zero block")
		// Replacing zeroeth data block (embedded in the root node)
		//TODO: check chunking here
		copy(dm.pbdata.Data[offset:], b)
		return len(b), nil
	}

	// Find where write should start
	var traversed uint64
	startsubblk := len(dm.pbdata.Blocksizes)
	if offset < zeroblocklen {
		dm.pbdata.Data = dm.pbdata.Data[:offset]
		startsubblk = 0
	} else {
		traversed = uint64(zeroblocklen)
		for i, size := range dm.pbdata.Blocksizes {
			if uint64(offset) < traversed+size {
				log.Debugf("Starting mod at block %d. [%d < %d + %d]", i, offset, traversed, size)
				// Here is where we start
				startsubblk = i
				lnk := dm.curNode.Links[i]
				node, err := dm.dagserv.Get(u.Key(lnk.Hash))
				if err != nil {
					return 0, err
				}
				data, err := ft.UnwrapData(node.Data)
				if err != nil {
					return 0, err
				}

				// We have to rewrite the data before our write in this block.
				b = append(data[:offset-traversed], b...)
				break
			}
			traversed += size
		}
		if startsubblk == len(dm.pbdata.Blocksizes) {
			// TODO: Im not sure if theres any case that isnt being handled here.
			// leaving this note here as a future reference in case something breaks
		}
	}

	// Find blocks that need to be overwritten
	var changed []int
	mid := -1
	var midoff uint64
	for i, size := range dm.pbdata.Blocksizes[startsubblk:] {
		if end > traversed {
			changed = append(changed, i+startsubblk)
		} else {
			break
		}
		traversed += size
		if end < traversed {
			mid = i + startsubblk
			midoff = end - (traversed - size)
			break
		}
	}

	// If our write starts in the middle of a block...
	var midlnk *mdag.Link
	if mid >= 0 {
		midlnk = dm.curNode.Links[mid]
		midnode, err := dm.dagserv.Get(u.Key(midlnk.Hash))
		if err != nil {
			return 0, err
		}

		// NOTE: this may have to be changed later when we have multiple
		// layers of indirection
		data, err := ft.UnwrapData(midnode.Data)
		if err != nil {
			return 0, err
		}
		b = append(b, data[midoff:]...)
	}

	// Generate new sub-blocks, and sizes
	subblocks := splitBytes(b, dm.splitter)
	var links []*mdag.Link
	var sizes []uint64
	for _, sb := range subblocks {
		n := &mdag.Node{Data: ft.WrapData(sb)}
		_, err := dm.dagserv.Add(n)
		if err != nil {
			log.Warningf("Failed adding node to DAG service: %s", err)
			return 0, err
		}
		lnk, err := mdag.MakeLink(n)
		if err != nil {
			return 0, err
		}
		links = append(links, lnk)
		sizes = append(sizes, uint64(len(sb)))
	}

	// This is disgusting (and can be rewritten if performance demands)
	if len(changed) > 0 {
		sechalflink := append(links, dm.curNode.Links[changed[len(changed)-1]+1:]...)
		dm.curNode.Links = append(dm.curNode.Links[:changed[0]], sechalflink...)
		sechalfblks := append(sizes, dm.pbdata.Blocksizes[changed[len(changed)-1]+1:]...)
		dm.pbdata.Blocksizes = append(dm.pbdata.Blocksizes[:changed[0]], sechalfblks...)
	} else {
		dm.curNode.Links = append(dm.curNode.Links, links...)
		dm.pbdata.Blocksizes = append(dm.pbdata.Blocksizes, sizes...)
	}
	dm.pbdata.Filesize = proto.Uint64(newsize)

	return origlen, nil
}

func (dm *DagModifier) Size() uint64 {
	if dm == nil {
		return 0
	}
	return dm.pbdata.GetFilesize()
}

// splitBytes uses a splitterFunc to turn a large array of bytes
// into many smaller arrays of bytes
func splitBytes(b []byte, spl chunk.BlockSplitter) [][]byte {
	out := spl.Split(bytes.NewReader(b))
	var arr [][]byte
	for blk := range out {
		arr = append(arr, blk)
	}
	return arr
}

// GetNode gets the modified DAG Node
func (dm *DagModifier) GetNode() (*mdag.Node, error) {
	b, err := proto.Marshal(dm.pbdata)
	if err != nil {
		return nil, err
	}
	dm.curNode.Data = b
	return dm.curNode.Copy(), nil
}
