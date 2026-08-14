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
// the kernel forgetting the entry and looking it up again. A rename carries
// the number over to the new name. Numbers are never reused: removing an
// entry retires its number, so a name that is created again is a new file
// with a new number, which is what a program watching the name expects.
//
// Only mutations that arrive through the mount retire a number. `ipfs files
// rm` on a mounted MFS goes straight to the same tree, so a name removed that
// way and created again keeps the number the deleted file had. Nothing is
// served wrong (each lookup still reads the live entry), but an observer
// watching st_ino alone does not see the replacement.
//
// The table only grows while a mount is up and starts empty on every mount,
// so inode numbers mean nothing outside the mount that issued them. It holds
// one entry per name looked up and not since removed through the mount, which
// includes every name a directory listing walks past, and on /ipns every
// external name that resolves.
//
// A lookup that is in flight while the name it is resolving is removed can
// leave its entry behind, since the allocation is not serialized against the
// removal. The entry is then either inherited by the next file of that name
// or held until unmount.
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
// Every node gets a generation of its own. go-fuse matches a lookup against
// the nodes it holds by all three fields together, so a fresh generation is
// what makes it build a new node rather than hand back one it already has for
// this entry. It has to: the node it kept has a *mfs.File or *mfs.Directory
// captured when it was built, and boxo replaces those whenever a directory
// flush clears its child cache. Reusing the node would mean operating on a
// handle MFS has moved on from, where `rm -r` unlinks entries that stay in
// the tree and writes go nowhere. A fresh node per lookup re-reads the
// handle, which is what the mount did before it numbered its own inodes, and
// all that changes is the number.
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

// move carries an entry's inode number over to the name it was renamed to and
// retires the number the destination held. Call it while both names are gone
// from MFS, between unlinking them and adding the entry back under the new
// name.
//
// The source keeps its number rather than being renumbered: two live entries
// can never share one, because numbers are only ever handed out fresh, and
// keeping it is what lets a program tell that the file it was watching moved
// instead of being replaced. A directory keeps its number for the same
// reason, and because its children are keyed by it: renumbering the directory
// would renumber everything under it and strand the old keys.
func (t *inodeTable) move(oldParent uint64, oldName string, newParent uint64, newName string) {
	oldKey := entryKey{parent: oldParent, name: oldName}
	newKey := entryKey{parent: newParent, name: newName}

	t.mu.Lock()
	defer t.mu.Unlock()

	ino, ok := t.entries[oldKey]
	delete(t.entries, oldKey)
	if !ok {
		// The source was never looked up, so it has no number to carry over
		// and the destination must not keep the one it had.
		delete(t.entries, newKey)
		return
	}
	t.entries[newKey] = ino
}
