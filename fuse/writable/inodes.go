// Inode numbering for writable mounts.
//
//go:build (linux || darwin || freebsd) && !nofuse

package writable

import (
	"sync"
	"sync/atomic"

	"github.com/hanwen/go-fuse/v2/fs"
)

// firstInode is the first number handed out to a directory entry. 0 is not
// a valid inode number and 1 is the mount root, so entries start above both.
const firstInode = 2

// entryKey identifies a directory entry by its parent's inode number and its
// name. MFS offers nothing better to key on: a file's CID changes on every
// write, and boxo hands out a fresh *mfs.File for an unchanged file whenever
// a directory flush clears its child cache.
type entryKey struct {
	parent uint64
	name   string
}

// inodeTable hands out inode numbers for the entries of one mount.
//
// go-fuse picks an automatic inode number for every Inode built with a zero
// StableAttr.Ino, and a different one each time. The kernel drops a mount's
// dentries once EntryTimeout expires, so under automatic numbering a file
// nobody touched comes back from the next lookup with a different st_ino.
// Programs that compare a file's identity over time then conclude the file
// was swapped underneath them: vim gives up on a save with "E949: File
// changed while writing".
//
// A number is allocated the first time an entry is looked up and survives
// the kernel forgetting the entry and looking it up again. Numbers are never
// reused. Removing or renaming an entry drops its number for good, so a name
// that is created again gets a new one. That is what keeps go-fuse from
// matching the new entry to the removed entry's Inode, which still holds a
// handle on the object that went away.
//
// The table only grows while a mount is up, by one entry per name looked up
// and not since removed, and it starts empty on every mount. Inode numbers
// therefore mean nothing outside the mount that issued them.
type inodeTable struct {
	mu      sync.Mutex
	next    uint64
	entries map[entryKey]uint64

	gen atomic.Uint64
}

func newInodeTable() *inodeTable {
	return &inodeTable{next: firstInode, entries: make(map[entryKey]uint64)}
}

// stable describes a node to go-fuse: which file it is (Ino), what kind of
// file (Mode), and which incarnation of it (Gen).
//
// Every node gets a generation of its own, which stops go-fuse from matching
// a lookup against a node it already holds for the same entry. Matching would
// be tempting, since the inode numbers now say the two are the same file, but
// the node go-fuse kept has a *mfs.File or *mfs.Directory captured when it was
// built, and boxo replaces those whenever a directory flush clears its child
// cache. Reusing the node would mean operating on a handle MFS has moved on
// from: `rm -r` unlinks entries that stay in the tree, and writes go nowhere.
// A fresh node per lookup re-reads the handle, which is what the mount did
// before it numbered its own inodes, and all that changes is the number.
func (t *inodeTable) stable(parent uint64, name string, mode uint32) fs.StableAttr {
	return fs.StableAttr{
		Ino:  t.get(parent, name),
		Mode: mode,
		Gen:  t.gen.Add(1),
	}
}

// get returns the inode number for an entry, allocating one on first use.
func (t *inodeTable) get(parent uint64, name string) uint64 {
	key := entryKey{parent: parent, name: name}

	t.mu.Lock()
	defer t.mu.Unlock()

	if ino, ok := t.entries[key]; ok {
		return ino
	}
	ino := t.next
	t.next++
	t.entries[key] = ino
	return ino
}

// InitInodes prepares the mount's inode table, once per Config. NewDir calls
// it, and so must any mount that serves entries of its own above the writable
// directories, before it answers a lookup. Call it while the mount is being
// built, not from a FUSE handler.
func (c *Config) InitInodes() {
	if c.inodes == nil {
		c.inodes = newInodeTable()
	}
}

// EntryAttr describes an entry of this mount to go-fuse: which entry it is
// (Ino) and what kind (Mode, file type bits only), plus a generation of its
// own. It is exported for the /ipns root, which serves the key directories
// and alias symlinks itself but has to number them from the same table as the
// files underneath them, or the two could hand out one number twice.
func (c *Config) EntryAttr(parent uint64, name string, mode uint32) fs.StableAttr {
	return c.inodes.stable(parent, name, mode)
}

// EntryIno returns an entry's inode number on its own, for filling
// fuse.DirEntry in a Readdir. See EntryAttr.
func (c *Config) EntryIno(parent uint64, name string) uint64 {
	return c.inodes.get(parent, name)
}

// drop forgets an entry's inode number. Call it once the entry is gone from
// MFS so that whatever takes the name next is numbered separately.
func (t *inodeTable) drop(parent uint64, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, entryKey{parent: parent, name: name})
}
