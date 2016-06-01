package filestore_util

import (
	errs "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipfs/go-ipfs/core"
	. "github.com/ipfs/go-ipfs/filestore"
	"github.com/ipfs/go-ipfs/unixfs"

	b "github.com/ipfs/go-ipfs/blocks/blockstore"
	bk "github.com/ipfs/go-ipfs/blocks/key"
	dag "github.com/ipfs/go-ipfs/merkledag"
)

type fileNodes map[bk.Key]struct{}

func (m fileNodes) have(key bk.Key) bool {
	_, ok := m[key]
	return ok
}

func (m fileNodes) add(key bk.Key) {
	m[key] = struct{}{}
}

// func extractFiles(key bk.Key, fs *Datastore, bs b.Blockservice, res *fileNodes) error {
// 	n, dataObj, status := getNode(key.DsKey(), key, fs, bs)
// 	if AnError(status) {
// 		return fmt.Errorf("Error when retrieving key: %s.", key)
// 	}
// 	if dataObj != nil {
// 		// already in filestore
// 		return nil
// 	}
// 	fsnode, err := unixfs.FromBytes(n.Data)
// 	if err != nil {
// 		return err
// 	}
// 	switch *fsnode.Type {
// 	case unixfs.TRaw:
// 	case unixfs.TFile:
// 		res.add(key)
// 	case unixfs.TDirectory:
// 		for _, link := range n.Links {
// 			err := extractFiles(bk.Key(link.Hash), fs, bs, res)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 	default:
// 	}
// 	return nil
// }

func ConvertToFile(node *core.IpfsNode, key bk.Key, path string) error {
	config, _ := node.Repo.Config()
	if node.OnlineMode() && (config == nil || !config.Filestore.APIServerSidePaths) {
		return errs.New("Node is online and server side paths are not enabled.")
	}
	if !filepath.IsAbs(path) {
		return errs.New("absolute path required")
	}
	wtr, err := os.Create(path)
	if err != nil {
		return err
	}
	fs, ok := node.Repo.SubDatastore(fsrepo.RepoFilestore).(*Datastore)
	if !ok {
		return errs.New("Could not extract filestore.")
	}
	p := params{node.Blockstore, fs, path, wtr}
	_, err = p.convertToFile(key, true, 0)
	return err
}

type params struct {
	bs   b.Blockstore
	fs   *Datastore
	path string
	out  io.Writer
}

func (p *params) convertToFile(key bk.Key, root bool, offset uint64) (uint64, error) {
	block, err := p.bs.Get(key)
	if err != nil {
		return 0, err
	}
	n, err := dag.DecodeProtobuf(block.Data())
	if err != nil {
		return 0, err
	}
	fsnode, err := unixfs.FSNodeFromBytes(n.Data)
	if fsnode.Type != unixfs.TRaw && fsnode.Type != unixfs.TFile {
		return 0, errs.New("Not a file")
	}
	dataObj := &DataObj{
		FilePath: p.path,
		Offset:   offset,
		Size:     fsnode.FileSize(),
	}
	if root {
		dataObj.Flags = WholeFile
	}
	if len(fsnode.Data) > 0 {
		_, err := p.out.Write(fsnode.Data)
		if err != nil {
			return 0, err
		}
		dataObj.Flags |= NoBlockData
		pbnode := n.GetPBNode()
		pbnode.Data, err = fsnode.GetBytesNoData()
		if err != nil {
			return 0, err
		}
		data, err := pbnode.Marshal()
		if err != nil {
			return 0, fmt.Errorf("Marshal failed. %v", err)
		}
		dataObj.Data = data
		p.fs.PutDirect(key.DsKey(), dataObj)
	} else {
		dataObj.Flags |= Internal
		dataObj.Data = block.Data()
		p.fs.PutDirect(key.DsKey(), dataObj)
		for _, link := range n.Links {
			size, err := p.convertToFile(bk.Key(link.Hash), false, offset)
			if err != nil {
				return 0, err
			}
			offset += size
		}
	}
	return fsnode.FileSize(), nil
}
