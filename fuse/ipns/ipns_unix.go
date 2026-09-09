//go:build (linux || darwin || freebsd) && !nofuse

// Package ipns implements a FUSE filesystem that interfaces with IPNS,
// the naming system for IPFS. Only names for which the node holds
// private keys are writable; all other names resolve to read-only
// symlinks pointing at the /ipfs mount.
package ipns

import (
	"context"
	"strings"
	"syscall"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	bstore "github.com/ipfs/boxo/blockstore"
	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
	mfs "github.com/ipfs/boxo/mfs"
	"github.com/ipfs/boxo/namesys"
	"github.com/ipfs/boxo/path"
	cid "github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	iface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/fuse/writable"
	"github.com/ipfs/kubo/internal/fusemount"
)

var log = logging.Logger("fuse/ipns")

// Root is the root object of the /ipns filesystem tree.
type Root struct {
	fs.Inode
	Ipfs iface.CoreAPI
	Keys map[string]iface.Key

	// Used for symlinking into ipfs
	IpfsRoot  string
	IpnsRoot  string
	LocalDirs map[string]*writable.Dir
	Roots     map[string]*mfs.Root

	LocalLinks map[string]*Link
	RepoPath   string

	// Cfg is the writable mounts' shared config. The root uses its inode
	// table so the entries it serves itself are numbered from the same
	// sequence as the files under the key directories.
	Cfg *writable.Config
}

func ipnsPubFunc(ipfs iface.CoreAPI, key iface.Key) mfs.PubFunc {
	return func(ctx context.Context, c cid.Cid) error {
		// Bypass the "cannot publish while IPNS is mounted" guard.
		// Without this the mount's own publishes are blocked,
		// causing silent data loss on daemon restart (issue #2168).
		ctx = fusemount.ContextWithPublish(ctx)
		_, err := ipfs.Name().Publish(ctx, path.FromCid(c), options.Name.Key(key.Name()), options.Name.AllowOffline(true))
		return err
	}
}

func loadRoot(ctx context.Context, ipfs iface.CoreAPI, key iface.Key, cfg *writable.Config, mfsOpts ...mfs.Option) (*mfs.Root, *writable.Dir, error) {
	node, err := ipfs.ResolveNode(ctx, key.Path())
	switch err {
	case nil:
	case namesys.ErrResolveFailed:
		node = ft.EmptyDirNode()
	default:
		log.Errorf("looking up %s: %s", key.Path(), err)
		return nil, nil, err
	}

	pbnode, ok := node.(*dag.ProtoNode)
	if !ok {
		return nil, nil, dag.ErrNotProtobuf
	}

	root, err := mfs.NewRoot(ctx, ipfs.Dag(), pbnode, ipnsPubFunc(ipfs, key), nil, mfsOpts...)
	if err != nil {
		return nil, nil, err
	}

	return root, writable.NewDir(root.GetDirectory(), cfg), nil
}

// CreateRoot creates the IPNS FUSE root with one writable directory per key.
func CreateRoot(ctx context.Context, ipfs iface.CoreAPI, gcLocker bstore.GCLocker, keys map[string]iface.Key, ipfspath, ipnspath, repoPath string, mountsCfg config.Mounts, imp config.Import, mfsOpts ...mfs.Option) (*Root, error) {
	cfg := &writable.Config{
		StoreMtime: mountsCfg.StoreMtime.WithDefault(config.DefaultStoreMtime),
		StoreMode:  mountsCfg.StoreMode.WithDefault(config.DefaultStoreMode),
		DAG:        ipfs.Dag(),
		RepoPath:   repoPath,
		Blksize:    fusemnt.BlksizeFromChunker(imp.UnixFSChunker.WithDefault(config.DefaultUnixFSChunker)),
		GCLocker:   gcLocker,
		MountCtx:   ctx,
	}
	// The root serves the key directories and alias symlinks itself, so it
	// needs the table even when there are no keys to build a Dir from.
	cfg.InitInodes()

	ldirs := make(map[string]*writable.Dir)
	roots := make(map[string]*mfs.Root)
	links := make(map[string]*Link)
	for alias, k := range keys {
		root, dir, err := loadRoot(ctx, ipfs, k, cfg, mfsOpts...)
		if err != nil {
			return nil, err
		}

		name := k.ID().String()
		roots[name] = root
		ldirs[name] = dir
		links[alias] = &Link{Target: name}
	}

	return &Root{
		Cfg:        cfg,
		Ipfs:       ipfs,
		IpfsRoot:   ipfspath,
		IpnsRoot:   ipnspath,
		Keys:       keys,
		LocalDirs:  ldirs,
		LocalLinks: links,
		Roots:      roots,
		RepoPath:   repoPath,
	}, nil
}

// Getattr returns the root directory attributes.
func (r *Root) Getattr(_ context.Context, _ fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr.Mode = uint32(fusemnt.NamespaceRootMode.Perm())
	out.Attr.Nlink = fusemnt.Nlink
	return 0
}

// Statfs reports disk-space statistics for the underlying filesystem.
// macOS Finder checks free space before copying; without this it
// reports "not enough free space" because go-fuse returns zeroed stats.
func (r *Root) Statfs(_ context.Context, out *fuse.StatfsOut) syscall.Errno {
	if r.RepoPath == "" {
		return 0
	}
	var s syscall.Statfs_t
	if err := syscall.Statfs(r.RepoPath, &s); err != nil {
		return fs.ToErrno(err)
	}
	out.FromStatfsT(&s)
	return 0
}

// fillEntryAttr copies a node's attributes into the lookup reply. Without it
// the reply carries zeroes, and since every later lookup refreshes the
// kernel's cache with the same zeroes, a key directory keeps showing up with
// no permissions and no link count.
func fillEntryAttr(ctx context.Context, node fs.NodeGetattrer, out *fuse.EntryOut) {
	var attr fuse.AttrOut
	if errno := node.Getattr(ctx, nil, &attr); errno != 0 {
		log.Warnf("ipns: attributes for lookup reply: %s", errno)
		return
	}
	out.Attr = attr.Attr
}

// entryAttr numbers an entry the root serves itself: a key directory, an
// alias symlink, or a symlink to a name resolved through IPNS. The number
// follows the name rather than what the name currently resolves to, so that
// a republished record does not renumber the entry.
func (r *Root) entryAttr(name string, mode uint32) fs.StableAttr {
	return r.Cfg.EntryAttr(fusemnt.RootIno, name, mode)
}

func (r *Root) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	switch name {
	case "mach_kernel", ".hidden", "._.":
		return nil, syscall.ENOENT
	}

	if lnk, ok := r.LocalLinks[name]; ok {
		fillEntryAttr(ctx, lnk, out)
		return r.NewInode(ctx, lnk, r.entryAttr(name, syscall.S_IFLNK)), 0
	}

	if dir, ok := r.LocalDirs[name]; ok {
		fillEntryAttr(ctx, dir, out)
		return r.NewInode(ctx, dir, r.entryAttr(name, syscall.S_IFDIR)), 0
	}

	// Other links go through IPNS resolution and are symlinked into the /ipfs mount.
	resolved, err := r.Ipfs.Name().Resolve(ctx, "/ipns/"+name)
	if err != nil {
		log.Warnf("ipns: namesys resolve error: %s", err)
		return nil, syscall.ENOENT
	}

	if resolved.Namespace() != path.IPFSNamespace {
		return nil, syscall.ENOENT
	}

	lnk := &Link{Target: r.IpfsRoot + "/" + strings.TrimPrefix(resolved.String(), "/ipfs/")}
	fillEntryAttr(ctx, lnk, out)
	return r.NewInode(ctx, lnk, r.entryAttr(name, syscall.S_IFLNK)), 0
}

func (r *Root) Readdir(_ context.Context) (fs.DirStream, syscall.Errno) {
	entries := make([]fuse.DirEntry, 0, len(r.Keys)*2)
	for alias, k := range r.Keys {
		keyName := k.ID().String()
		entries = append(entries,
			fuse.DirEntry{Name: keyName, Mode: syscall.S_IFDIR, Ino: r.Cfg.EntryIno(fusemnt.RootIno, keyName)},
			fuse.DirEntry{Name: alias, Mode: syscall.S_IFLNK, Ino: r.Cfg.EntryIno(fusemnt.RootIno, alias)},
		)
	}
	return fs.NewListDirStream(entries), 0
}

func (r *Root) Close() error {
	for _, mr := range r.Roots {
		if err := mr.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Interface compliance checks for Root.
var (
	_ fs.NodeGetattrer = (*Root)(nil)
	_ fs.NodeLookuper  = (*Root)(nil)
	_ fs.NodeReaddirer = (*Root)(nil)
	_ fs.NodeStatfser  = (*Root)(nil)
)
