// A Go mirror of libfuse's hello.c

package readonly

import (
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"github.com/hanwen/go-fuse/fuse/pathfs"
	core "github.com/jbenet/go-ipfs/core"
)

type FileSystem struct {
	Ipfs *core.IpfsNode
	pathfs.FileSystem
}

func NewFileSystem(ipfs *core.IpfsNode) *FileSystem {
	return &FileSystem{
		Ipfs:       ipfs,
		FileSystem: pathfs.NewDefaultFileSystem(),
	}
}

func (s *FileSystem) GetAttr(name string, context *fuse.Context) (*fuse.Attr, fuse.Status) {
	if name == "/" { // -rw +x on root
		return &fuse.Attr{Mode: fuse.S_IFDIR | 0111}, fuse.OK
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	// links? say dir. could have data...
	if len(nd.Links) > 0 {
		return &fuse.Attr{Mode: fuse.S_IFDIR | 0555}, fuse.OK
	}

	// size
	size, _ := nd.Size()

	// file.
	return &fuse.Attr{
		Mode: fuse.S_IFREG | 0444,
		Size: uint64(size),
	}, fuse.OK
}

func (s *FileSystem) OpenDir(name string, context *fuse.Context) (c []fuse.DirEntry, code fuse.Status) {
	if name == "/" { // nope
		return nil, fuse.EPERM
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	entries := make([]fuse.DirEntry, len(nd.Links))
	for i, link := range nd.Links {
		n := link.Name
		if len(n) == 0 {
			n = link.Hash.B58String()
		}
		entries[i] = fuse.DirEntry{Name: n, Mode: fuse.S_IFREG | 0444}
	}

	if len(entries) > 0 {
		return entries, fuse.OK
	}
	return nil, fuse.ENOENT
}

func (s *FileSystem) Open(name string, flags uint32, context *fuse.Context) (
	file nodefs.File, code fuse.Status) {

	// read only, bro!
	if flags&fuse.O_ANYWRITE != 0 {
		return nil, fuse.EPERM
	}

	nd, err := s.Ipfs.Resolver.ResolvePath(name)
	if err != nil {
		// todo: make this error more versatile.
		return nil, fuse.ENOENT
	}

	return nodefs.NewDataFile([]byte(nd.Data)), fuse.OK
}

func (s *FileSystem) String() string {
  return "IpfsReadOnly"
}

func (s *FileSystem) OnMount(nodeFs *pathfs.PathNodeFs) {
}

func Mount(s *FileSystem, path string) (*fuse.Server, error) {
  rfs := pathfs.NewReadonlyFileSystem(s)
	fs := pathfs.NewPathNodeFs(rfs, nil)
	ser, _, err := nodefs.MountRoot(path, fs.Root(), nil)
	return ser, err
}
