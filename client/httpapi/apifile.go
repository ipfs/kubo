package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core"
)

const forwardSeekLimit = 1 << 14 //16k

func (api *UnixfsAPI) Get(ctx context.Context, p iface.Path) (files.Node, error) {
	if p.Mutable() { // use resolved path in case we are dealing with IPNS / MFS
		var err error
		p, err = api.core().ResolvePath(ctx, p)
		if err != nil {
			return nil, err
		}
	}

	var stat struct {
		Hash string
		Type string
		Size int64 // unixfs size
	}
	err := api.core().request("files/stat", p.String()).Exec(ctx, &stat)
	if err != nil {
		return nil, err
	}

	switch stat.Type {
	case "file":
		return api.getFile(ctx, p, stat.Size)
	case "directory":
		return api.getDir(ctx, p, stat.Size)
	default:
		return nil, fmt.Errorf("unsupported file type '%s'", stat.Type)
	}
}

type apiFile struct {
	ctx  context.Context
	core *HttpApi
	size int64
	path iface.Path

	r  io.ReadCloser
	at int64
}

func (f *apiFile) reset() error {
	if f.r != nil {
		f.r.Close()
	}
	req := f.core.request("cat", f.path.String()).NoDrain()
	if f.at != 0 {
		req.Option("offset", f.at)
	}
	resp, err := req.Send(f.ctx)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	f.r = resp.Output
	return nil
}

func (f *apiFile) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if n > 0 {
		f.at += int64(n)
	}
	return n, err
}

func (f *apiFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekEnd:
		offset = f.size + offset
	case io.SeekCurrent:
		offset = f.at + offset
	}
	if f.at == offset { //noop
		return offset, nil
	}

	if f.at < offset && offset-f.at < forwardSeekLimit { //forward skip
		r, err := io.CopyN(ioutil.Discard, f.r, offset-f.at)

		f.at += r
		return f.at, err
	}
	f.at = offset
	return f.at, f.reset()
}

func (f *apiFile) Close() error {
	if f.r != nil {
		return f.r.Close()
	}
	return nil
}

func (f *apiFile) Size() (int64, error) {
	return f.size, nil
}

func (api *UnixfsAPI) getFile(ctx context.Context, p iface.Path, size int64) (files.Node, error) {
	f := &apiFile{
		ctx:  ctx,
		core: api.core(),
		size: size,
		path: p,
	}

	return f, f.reset()
}

type apiIter struct {
	ctx  context.Context
	core *UnixfsAPI

	err error

	dec     *json.Decoder
	curFile files.Node
	cur     lsLink
}

func (it *apiIter) Err() error {
	return it.err
}

func (it *apiIter) Name() string {
	return it.cur.Name
}

func (it *apiIter) Next() bool {
	if it.ctx.Err() != nil {
		it.err = it.ctx.Err()
		return false
	}

	var out lsOutput
	if err := it.dec.Decode(&out); err != nil {
		if err != io.EOF {
			it.err = err
		}
		return false
	}

	if len(out.Objects) != 1 {
		it.err = fmt.Errorf("ls returned more objects than expected (%d)", len(out.Objects))
		return false
	}

	if len(out.Objects[0].Links) != 1 {
		it.err = fmt.Errorf("ls returned more links than expected (%d)", len(out.Objects[0].Links))
		return false
	}

	it.cur = out.Objects[0].Links[0]
	c, err := cid.Parse(it.cur.Hash)
	if err != nil {
		it.err = err
		return false
	}

	switch it.cur.Type {
	case iface.THAMTShard:
		fallthrough
	case iface.TMetadata:
		fallthrough
	case iface.TDirectory:
		it.curFile, err = it.core.getDir(it.ctx, iface.IpfsPath(c), int64(it.cur.Size))
		if err != nil {
			it.err = err
			return false
		}
	case iface.TFile:
		it.curFile, err = it.core.getFile(it.ctx, iface.IpfsPath(c), int64(it.cur.Size))
		if err != nil {
			it.err = err
			return false
		}
	default:
		it.err = fmt.Errorf("file type %d not supported", it.cur.Type)
		return false
	}
	return true
}

func (it *apiIter) Node() files.Node {
	return it.curFile
}

type apiDir struct {
	ctx  context.Context
	core *UnixfsAPI
	size int64
	path iface.Path

	dec *json.Decoder
}

func (d *apiDir) Close() error {
	return nil
}

func (d *apiDir) Size() (int64, error) {
	return d.size, nil
}

func (d *apiDir) Entries() files.DirIterator {
	return &apiIter{
		ctx:  d.ctx,
		core: d.core,
		dec:  d.dec,
	}
}

func (api *UnixfsAPI) getDir(ctx context.Context, p iface.Path, size int64) (files.Node, error) {
	resp, err := api.core().request("ls", p.String()).
		Option("resolve-size", true).
		Option("stream", true).Send(ctx)

	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	d := &apiDir{
		ctx:  ctx,
		core: api,
		size: size,
		path: p,

		dec: json.NewDecoder(resp.Output),
	}

	return d, nil
}

var _ files.File = &apiFile{}
var _ files.Directory = &apiDir{}
