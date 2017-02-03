package filestore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ipfs/go-ipfs/blocks"
	"github.com/ipfs/go-ipfs/blocks/blockstore"
	pb "github.com/ipfs/go-ipfs/filestore/pb"
	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	posinfo "github.com/ipfs/go-ipfs/thirdparty/posinfo"

	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	dsns "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/namespace"
	dsq "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/query"
	proto "gx/ipfs/QmT6n4mspWYEya864BhCUJEgyxiRfmiSY9ruQwTUNpRKaM/protobuf/proto"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
)

var FilestorePrefix = ds.NewKey("filestore")

type FileManager struct {
	ds   ds.Batching
	root string
}

type CorruptReferenceError struct {
	Err error
}

func (c CorruptReferenceError) Error() string {
	return c.Err.Error()
}

func NewFileManager(ds ds.Batching, root string) *FileManager {
	return &FileManager{dsns.Wrap(ds, FilestorePrefix), root}
}

func (f *FileManager) AllKeysChan(ctx context.Context) (<-chan *cid.Cid, error) {
	q := dsq.Query{KeysOnly: true}
	q.Prefix = FilestorePrefix.String()

	res, err := f.ds.Query(q)
	if err != nil {
		return nil, err
	}

	out := make(chan *cid.Cid, dsq.KeysOnlyBufSize)
	go func() {
		defer close(out)
		for {
			v, ok := res.NextSync()
			if !ok {
				return
			}

			k := ds.RawKey(v.Key)
			c, err := dshelp.DsKeyToCid(k)
			if err != nil {
				log.Error("decoding cid from filestore: %s", err)
				continue
			}

			select {
			case out <- c:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}

func (f *FileManager) DeleteBlock(c *cid.Cid) error {
	err := f.ds.Delete(dshelp.CidToDsKey(c))
	if err == ds.ErrNotFound {
		return blockstore.ErrNotFound
	}
	return err
}

func (f *FileManager) Get(c *cid.Cid) (blocks.Block, error) {
	dobj, err := f.getDataObj(c)
	if err != nil {
		return nil, err
	}

	out, err := f.readDataObj(c, dobj)
	if err != nil {
		return nil, err
	}

	return blocks.NewBlockWithCid(out, c)
}

func (f *FileManager) getDataObj(c *cid.Cid) (*pb.DataObj, error) {
	o, err := f.ds.Get(dshelp.CidToDsKey(c))
	switch err {
	case ds.ErrNotFound:
		return nil, blockstore.ErrNotFound
	default:
		return nil, err
	case nil:
		//
	}

	data, ok := o.([]byte)
	if !ok {
		return nil, fmt.Errorf("stored filestore dataobj was not a []byte")
	}

	var dobj pb.DataObj
	if err := proto.Unmarshal(data, &dobj); err != nil {
		return nil, err
	}

	return &dobj, nil
}

// reads and verifies the block
func (f *FileManager) readDataObj(c *cid.Cid, d *pb.DataObj) ([]byte, error) {
	p := filepath.FromSlash(d.GetFilePath())
	abspath := filepath.Join(f.root, p)

	fi, err := os.Open(abspath)
	if err != nil {
		return nil, &CorruptReferenceError{err}
	}
	defer fi.Close()

	_, err = fi.Seek(int64(d.GetOffset()), os.SEEK_SET)
	if err != nil {
		return nil, &CorruptReferenceError{err}
	}

	outbuf := make([]byte, d.GetSize_())
	_, err = io.ReadFull(fi, outbuf)
	if err != nil {
		return nil, &CorruptReferenceError{err}
	}

	outcid, err := c.Prefix().Sum(outbuf)
	if err != nil {
		return nil, err
	}

	if !c.Equals(outcid) {
		return nil, &CorruptReferenceError{fmt.Errorf("data in file did not match. %s offset %d", d.GetFilePath(), d.GetOffset())}
	}

	return outbuf, nil
}

func (f *FileManager) Has(c *cid.Cid) (bool, error) {
	// NOTE: interesting thing to consider. Has doesnt validate the data.
	// So the data on disk could be invalid, and we could think we have it.
	dsk := dshelp.CidToDsKey(c)
	return f.ds.Has(dsk)
}

type putter interface {
	Put(ds.Key, interface{}) error
}

func (f *FileManager) Put(b *posinfo.FilestoreNode) error {
	return f.putTo(b, f.ds)
}

func (f *FileManager) putTo(b *posinfo.FilestoreNode, to putter) error {
	var dobj pb.DataObj

	if !filepath.HasPrefix(b.PosInfo.FullPath, f.root) {
		return fmt.Errorf("cannot add filestore references outside ipfs root")
	}

	p, err := filepath.Rel(f.root, b.PosInfo.FullPath)
	if err != nil {
		return err
	}

	dobj.FilePath = proto.String(filepath.ToSlash(p))
	dobj.Offset = proto.Uint64(b.PosInfo.Offset)
	dobj.Size_ = proto.Uint64(uint64(len(b.RawData())))

	data, err := proto.Marshal(&dobj)
	if err != nil {
		return err
	}

	return to.Put(dshelp.CidToDsKey(b.Cid()), data)
}

func (f *FileManager) PutMany(bs []*posinfo.FilestoreNode) error {
	batch, err := f.ds.Batch()
	if err != nil {
		return err
	}

	for _, b := range bs {
		if err := f.putTo(b, batch); err != nil {
			return err
		}
	}

	return batch.Commit()
}
