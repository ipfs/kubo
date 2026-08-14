// Reusable test suite for writable FUSE mounts.
//
// RunWritableSuite exercises all filesystem operations shared by
// /mfs and /ipns. Each mount provides a MountFunc that creates a
// fresh writable mount.
//
//go:build (linux || darwin || freebsd) && !nofuse

package fusetest

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"

	racedet "github.com/ipfs/go-detect-race"
	fusemnt "github.com/ipfs/kubo/fuse/mount"
	"github.com/ipfs/kubo/fuse/writable"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// MountFunc creates a fresh writable FUSE mount and returns the root
// directory path. Cleanup is handled via t.Cleanup.
type MountFunc func(t *testing.T, cfg writable.Config) string

// RunWritableSuite runs generic writable filesystem tests against
// the mount produced by mount.
func RunWritableSuite(t *testing.T, mount MountFunc) {
	t.Run("ReadWrite", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		data := WriteFileOrFail(t, 500, filepath.Join(dir, "testfile"))
		VerifyFile(t, filepath.Join(dir, "testfile"), data)
	})

	t.Run("AppendFile", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "appendme")

		part1 := RandBytes(200)
		require.NoError(t, os.WriteFile(path, part1, 0o644))

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
		require.NoError(t, err)
		part2 := RandBytes(300)
		_, err = f.Write(part2)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		VerifyFile(t, path, append(part1, part2...))
	})

	t.Run("MultiWrite", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "multiwrite")

		f, err := os.Create(path)
		require.NoError(t, err)
		var want []byte
		for range 1001 {
			b := []byte{byte(mrand.Intn(256))}
			_, err := f.Write(b)
			require.NoError(t, err)
			want = append(want, b...)
		}
		require.NoError(t, f.Close())
		VerifyFile(t, path, want)
	})

	t.Run("EmptyDirListing", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		emptyDir := filepath.Join(dir, "emptydir")
		require.NoError(t, os.Mkdir(emptyDir, 0o755))

		entries, err := os.ReadDir(emptyDir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})

	t.Run("Mkdir", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		nested := filepath.Join(dir, "a", "b", "c")
		require.NoError(t, os.MkdirAll(nested, 0o755))

		info, err := os.Stat(nested)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})

	// Both fstat (on the open handle) and path-based stat must return
	// the correct mode and size for a freshly created file. The kernel
	// caches attrs from the Create response for AttrTimeout: if
	// Dir.Create returns an empty EntryOut.Attr, fstat sees the cached
	// zero values. A path-based stat does a fresh Lookup, which has its
	// own attr-fill path; covering both shapes guards against future
	// regressions on either side.
	t.Run("CreateAttrsImmediate", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "freshfile")

		f, err := os.Create(path)
		require.NoError(t, err)
		defer f.Close()

		// fstat on the open handle: exercises the Create response cache.
		fstatInfo, err := f.Stat()
		require.NoError(t, err)
		require.Equal(t, int64(0), fstatInfo.Size())
		require.Equal(t, os.FileMode(0o644), fstatInfo.Mode().Perm(),
			"fstat on new file should report default mode, not cached zero")

		// Path-based stat: exercises Dir.Lookup → FileInode.fillAttr.
		statInfo, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, int64(0), statInfo.Size())
		require.Equal(t, os.FileMode(0o644), statInfo.Mode().Perm(),
			"stat on new file should report default mode, not cached zero")
	})

	// Same as CreateAttrsImmediate, but for mkdir. Mkdir does not return
	// a file handle, so we open the directory afterwards and fstat its
	// fd to exercise the inode-level path. Path-based stat exercises
	// Lookup. Both must report the directory mode.
	t.Run("MkdirAttrsImmediate", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "freshdir")

		require.NoError(t, os.Mkdir(path, 0o755))

		// Path-based stat: exercises Dir.Lookup → Dir.fillAttr.
		statInfo, err := os.Stat(path)
		require.NoError(t, err)
		require.True(t, statInfo.IsDir())
		require.Equal(t, os.FileMode(0o755), statInfo.Mode().Perm(),
			"stat on new directory should report default mode, not cached zero")

		// fstat on an open directory fd: exercises Dir.Getattr.
		f, err := os.Open(path)
		require.NoError(t, err)
		defer f.Close()
		fstatInfo, err := f.Stat()
		require.NoError(t, err)
		require.True(t, fstatInfo.IsDir())
		require.Equal(t, os.FileMode(0o755), fstatInfo.Mode().Perm(),
			"fstat on new directory should report default mode, not cached zero")
	})

	t.Run("RenameFile", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		src := filepath.Join(dir, "oldname")
		dst := filepath.Join(dir, "newname")

		data := WriteFileOrFail(t, 300, src)
		require.NoError(t, os.Rename(src, dst))

		_, err := os.Stat(src)
		require.True(t, os.IsNotExist(err))
		VerifyFile(t, dst, data)
	})

	t.Run("CrossDirRename", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		require.NoError(t, os.Mkdir(filepath.Join(dir, "src"), 0o755))
		require.NoError(t, os.Mkdir(filepath.Join(dir, "dst"), 0o755))

		data := WriteFileOrFail(t, 200, filepath.Join(dir, "src", "file"))
		require.NoError(t, os.Rename(filepath.Join(dir, "src", "file"), filepath.Join(dir, "dst", "file")))

		_, err := os.Stat(filepath.Join(dir, "src", "file"))
		require.True(t, os.IsNotExist(err))
		VerifyFile(t, filepath.Join(dir, "dst", "file"), data)
	})

	// Renaming a directory (not just a file inside it). The contained
	// file must still be readable under the new path.
	t.Run("DirRename", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		oldDir := filepath.Join(dir, "olddir")
		newDir := filepath.Join(dir, "newdir")

		require.NoError(t, os.Mkdir(oldDir, 0o755))
		data := WriteFileOrFail(t, 200, filepath.Join(oldDir, "child"))

		require.NoError(t, os.Rename(oldDir, newDir))

		_, err := os.Stat(oldDir)
		require.True(t, os.IsNotExist(err))
		VerifyFile(t, filepath.Join(newDir, "child"), data)
	})

	t.Run("RemoveFile", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "removeme")
		WriteFileOrFail(t, 100, path)
		require.NoError(t, os.Remove(path))

		_, err := os.Stat(path)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("Rmdir", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		sub := filepath.Join(dir, "rmdir_target")
		require.NoError(t, os.Mkdir(sub, 0o755))
		require.NoError(t, os.Remove(sub))

		_, err := os.Stat(sub)
		require.True(t, os.IsNotExist(err))
	})

	// A rename may only replace a directory when that directory is empty.
	// MFS removes a directory and everything under it without complaint, so
	// the mount has to check: `mv -T src dst` used to take dst's contents
	// with it. mv(1) does its own checking, so drive rename(2) directly.
	t.Run("RenameOntoNonEmptyDirectory", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		src := filepath.Join(dir, "rename_src")
		dst := filepath.Join(dir, "rename_dst")
		require.NoError(t, os.Mkdir(src, 0o755))
		require.NoError(t, os.Mkdir(dst, 0o755))
		moved := WriteFileOrFail(t, 50, filepath.Join(src, "moved"))
		keep := WriteFileOrFail(t, 50, filepath.Join(dst, "keep"))

		require.ErrorIs(t, syscall.Rename(src, dst), syscall.ENOTEMPTY)

		VerifyFile(t, filepath.Join(dst, "keep"), keep)
		VerifyFile(t, filepath.Join(src, "moved"), moved)
	})

	t.Run("RemoveNonEmptyDirectory", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		sub := filepath.Join(dir, "nonempty")
		require.NoError(t, os.Mkdir(sub, 0o755))
		WriteFileOrFail(t, 50, filepath.Join(sub, "child"))

		err := syscall.Rmdir(sub)
		require.Error(t, err, "expected error removing non-empty directory")

		// After removing the child, rmdir succeeds.
		require.NoError(t, os.Remove(filepath.Join(sub, "child")))
		require.NoError(t, os.Remove(sub))
	})

	t.Run("DoubleEntryFailure", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		sub := filepath.Join(dir, "dupdir")
		require.NoError(t, os.Mkdir(sub, 0o755))
		require.Error(t, os.Mkdir(sub, 0o755))
	})

	t.Run("Fsync", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "fsyncme")

		f, err := os.Create(path)
		require.NoError(t, err)
		_, err = f.Write(RandBytes(500))
		require.NoError(t, err)
		require.NoError(t, f.Sync())
		require.NoError(t, f.Close())
	})

	// After fsync on the writer handle, a fresh reader on a different
	// fd must see the synced data. This is the "vim wrote and called
	// fsync; my other process should see it immediately" scenario.
	t.Run("FsyncCrossHandle", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "fsynccross")

		want := RandBytes(500)
		w, err := os.Create(path)
		require.NoError(t, err)
		_, err = w.Write(want)
		require.NoError(t, err)
		require.NoError(t, w.Sync())
		// w is intentionally still open: the cross-handle reader must
		// see the data after fsync, not just after close.

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, len(want), len(got),
			"reader on fresh handle should see all bytes after fsync")
		require.Equal(t, want, got,
			"reader on a fresh handle should see data flushed by fsync")

		require.NoError(t, w.Close())
	})

	t.Run("Ftruncate", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "truncme")

		f, err := os.Create(path)
		require.NoError(t, err)
		_, err = f.Write(RandBytes(1000))
		require.NoError(t, err)
		require.NoError(t, f.Truncate(500))
		require.NoError(t, f.Close())

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, int64(500), info.Size())
	})

	// truncate(path, size) without an open fd: uses a temporary
	// write descriptor inside Setattr instead of ftruncate on an
	// existing handle.
	t.Run("TruncatePath", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "pathtrunc")

		WriteFileOrFail(t, 1000, path)
		require.NoError(t, syscall.Truncate(path, 500))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, int64(500), info.Size())
	})

	t.Run("LargeFile", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "largefile")
		size := 1024*1024 + 1 // 1 MiB + 1 byte
		data := WriteFileOrFail(t, size, path)
		VerifyFile(t, path, data)
	})

	t.Run("OpenTrunc", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "truncopen")

		WriteFileOrFail(t, 500, path)

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		newData := RandBytes(200)
		_, err = f.Write(newData)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		VerifyFile(t, path, newData)
	})

	t.Run("TempFileRename", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		target := filepath.Join(dir, "target")
		tmp := filepath.Join(dir, ".target.tmp")

		WriteFileOrFail(t, 100, target)
		newData := WriteFileOrFail(t, 200, tmp)
		require.NoError(t, os.Rename(tmp, target))

		VerifyFile(t, target, newData)
	})

	t.Run("SeekAndWrite", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "seekwrite")
		data := WriteFileOrFail(t, 100, path)

		f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
		require.NoError(t, err)
		patch := []byte("PATCHED")
		_, err = f.WriteAt(patch, 10)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		copy(data[10:], patch)
		VerifyFile(t, path, data)
	})

	// Writing past the end of an empty file. UnixFS may not store true
	// sparse holes, but the visible read must report the requested
	// offset and the data we wrote, with zero bytes filling the gap.
	t.Run("SparseWrite", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "sparse")

		f, err := os.Create(path)
		require.NoError(t, err)
		payload := RandBytes(100)
		_, err = f.WriteAt(payload, 1000)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, 1100, len(got), "size should include the gap before the written bytes")
		require.True(t, bytes.Equal(payload, got[1000:]), "tail bytes should match the written payload")
		// Bytes [0:1000] should read as zero. Don't assert byte-for-byte
		// equality with a zero slice (would catch the same thing twice);
		// require.NotContains over a sample is enough.
		for _, b := range got[:1000] {
			if b != 0 {
				t.Fatalf("expected zero gap fill, got byte %d", b)
			}
		}
	})

	// O_EXCL: the second create on the same path must fail with an
	// error that satisfies os.IsExist. Lock files, ssh-agent, and
	// atomic file creation patterns rely on this.
	t.Run("OExcl", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "exclfile")

		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		_, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		require.Error(t, err)
		require.True(t, os.IsExist(err), "second O_EXCL create should fail with EEXIST, got %v", err)
	})

	t.Run("OverwriteExisting", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "overwrite")

		WriteFileOrFail(t, 500, path)

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		newData := RandBytes(300)
		_, err = f.Write(newData)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		VerifyFile(t, path, newData)
	})

	// Vim (with backupcopy=yes) save sequence: open O_TRUNC, write, fsync, chmod.
	t.Run("VimSavePattern", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMode: true})
		path := filepath.Join(dir, "vimsave")

		WriteFileOrFail(t, 200, path)

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
		require.NoError(t, err)
		newData := RandBytes(300)
		_, err = f.Write(newData)
		require.NoError(t, err)
		require.NoError(t, f.Sync())
		require.NoError(t, f.Chmod(0o644))
		require.NoError(t, f.Close())

		VerifyFile(t, path, newData)
	})

	// A file's inode number has to outlive the kernel dropping its directory
	// entry and looking it up again, which happens once EntryTimeout passes.
	// When it does not, a program that compares a file's identity over time
	// concludes the file was swapped underneath it: vim abandons a save with
	// "E949: File changed while writing".
	t.Run("InodeNumbers", func(t *testing.T) {
		dir := mount(t, writable.Config{})

		var mnt unix.Stat_t
		require.NoError(t, unix.Stat(dir, &mnt))
		require.NotZero(t, mnt.Ino, "mount point should report an inode number")

		path := filepath.Join(dir, "stable")
		require.NoError(t, os.WriteFile(path, []byte("first"), 0o644))

		var first unix.Stat_t
		require.NoError(t, unix.Stat(path, &first))
		require.NotZero(t, first.Ino, "file should report an inode number")
		// go-fuse numbers whatever the filesystem leaves to it from
		// AutomaticIno up, handing out a new number for every node it builds.
		// A number in that range means the mount is not numbering its own
		// entries.
		require.Less(t, first.Ino, uint64(fusemnt.AutomaticIno),
			"inode number should come from the mount, not go-fuse's automatic range")

		// Outlast the entry timeout of the writable mounts (one second) so
		// the kernel has to ask for the entry again.
		time.Sleep(1500 * time.Millisecond)

		var again unix.Stat_t
		require.NoError(t, unix.Stat(path, &again))
		require.Equal(t, first.Ino, again.Ino, "inode number should survive a re-lookup")

		// A name that is removed and created again names a different file, so
		// it must not inherit the old number. go-fuse matches nodes by inode
		// number, and would hand back the removed entry's node; MFS has that
		// one marked as unlinked and drops writes made through it.
		require.NoError(t, os.Remove(path))
		require.NoError(t, os.WriteFile(path, []byte("second"), 0o644))

		var recreated unix.Stat_t
		require.NoError(t, unix.Stat(path, &recreated))
		require.NotEqual(t, first.Ino, recreated.Ino,
			"a name that was removed and created again should get a new inode number")
		VerifyFile(t, path, []byte("second"))

		// A directory listing reports an inode number per entry of its own,
		// and tools that read it (ls -i, find) never learn of the one stat
		// would give them. The two have to agree.
		assertDirEntryInos(t, dir)
	})

	// Renaming moves a file, it does not replace it, so the inode number goes
	// with it. Backup tools that track files by identity (tar
	// --listed-incremental, file watchers) re-copy a file whose number
	// changed, and a directory rename must not renumber what is inside it.
	t.Run("InodeNumbersAcrossRename", func(t *testing.T) {
		dir := mount(t, writable.Config{})

		src := filepath.Join(dir, "before")
		require.NoError(t, os.WriteFile(src, []byte("payload"), 0o644))

		subdir := filepath.Join(dir, "olddir")
		require.NoError(t, os.Mkdir(subdir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(subdir, "child"), []byte("payload"), 0o644))

		ino := func(path string) uint64 {
			t.Helper()
			var st unix.Stat_t
			require.NoError(t, unix.Stat(path, &st))
			return st.Ino
		}

		fileBefore := ino(src)
		dirBefore := ino(subdir)
		childBefore := ino(filepath.Join(subdir, "child"))

		dst := filepath.Join(dir, "after")
		moved := filepath.Join(dir, "newdir")
		require.NoError(t, os.Rename(src, dst))
		require.NoError(t, os.Rename(subdir, moved))

		require.Equal(t, fileBefore, ino(dst), "a renamed file keeps its inode number")
		require.Equal(t, dirBefore, ino(moved), "a renamed directory keeps its inode number")

		// The kernel answered those from the directory entries it moved
		// itself. Outlasting the entry timeout makes it ask the mount, which
		// is where a renamed entry used to come back as a different file.
		time.Sleep(1500 * time.Millisecond)

		require.Equal(t, fileBefore, ino(dst), "and keeps it once the kernel asks again")
		require.Equal(t, dirBefore, ino(moved), "and so does the directory")
		require.Equal(t, childBefore, ino(filepath.Join(moved, "child")),
			"an entry inside a renamed directory keeps its inode number")

		// The renamed directory is a live MFS handle, not the one that was
		// unlinked: writes through it have to reach the tree.
		child := filepath.Join(moved, "child")
		require.NoError(t, os.WriteFile(child, []byte("rewritten"), 0o644))
		VerifyFile(t, child, []byte("rewritten"))
	})

	// A rename leaves the kernel holding the entry it moved, and that entry
	// carries the MFS handle the rename unlinked. Writing through the new
	// name straight away has to reach the tree; it used to be accepted and
	// then dropped, and the file read back with its old contents once the
	// entry cache expired.
	t.Run("WriteAfterRename", func(t *testing.T) {
		dir := mount(t, writable.Config{})

		src := filepath.Join(dir, "before")
		dst := filepath.Join(dir, "after")
		require.NoError(t, os.WriteFile(src, []byte("one"), 0o644))
		require.NoError(t, os.Rename(src, dst))
		require.NoError(t, os.WriteFile(dst, []byte("two"), 0o644))

		// Outlast the entry timeout so the kernel asks the mount again
		// instead of answering from the node it moved.
		time.Sleep(1500 * time.Millisecond)
		VerifyFile(t, dst, []byte("two"))

		// Same for a directory: an entry created under the new name has to
		// land there, and the old name must not come back.
		oldDir := filepath.Join(dir, "olddir2")
		newDir := filepath.Join(dir, "newdir2")
		require.NoError(t, os.Mkdir(oldDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(oldDir, "kept"), []byte("kept"), 0o644))
		require.NoError(t, os.Rename(oldDir, newDir))
		require.NoError(t, os.WriteFile(filepath.Join(newDir, "fresh"), []byte("fresh"), 0o644))

		time.Sleep(1500 * time.Millisecond)
		VerifyFile(t, filepath.Join(newDir, "fresh"), []byte("fresh"))
		VerifyFile(t, filepath.Join(newDir, "kept"), []byte("kept"))
		_, err := os.Stat(oldDir)
		require.True(t, os.IsNotExist(err), "the name a rename moved away from must stay gone")

		// And an entry the kernel had already looked up before the rename.
		// Its handle hangs off the directory the rename replaced, a level
		// below the entry that moved.
		oldParent := filepath.Join(dir, "oldparent")
		newParent := filepath.Join(dir, "newparent")
		require.NoError(t, os.Mkdir(oldParent, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(oldParent, "child"), []byte("one"), 0o644))
		VerifyFile(t, filepath.Join(oldParent, "child"), []byte("one")) // make the kernel hold it

		require.NoError(t, os.Rename(oldParent, newParent))
		require.NoError(t, os.WriteFile(filepath.Join(newParent, "child"), []byte("two"), 0o644))

		time.Sleep(1500 * time.Millisecond)
		VerifyFile(t, filepath.Join(newParent, "child"), []byte("two"))
	})

	// rsync default save: create temp file, write, rename over target.
	t.Run("RsyncPattern", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		target := filepath.Join(dir, "rsync_target")
		tmp := filepath.Join(dir, ".rsync_target.XXXXXX")

		WriteFileOrFail(t, 100, target)
		newData := WriteFileOrFail(t, 200, tmp)
		require.NoError(t, os.Rename(tmp, target))

		VerifyFile(t, target, newData)
	})

	t.Run("Symlink", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		link := filepath.Join(dir, "mylink")
		require.NoError(t, os.Symlink("/some/target", link))

		got, err := os.Readlink(link)
		require.NoError(t, err)
		require.Equal(t, "/some/target", got)
	})

	// Verify that readdir reports symlinks with ModeSymlink so that
	// tools like ls -l and find -type l see the correct file type.
	t.Run("SymlinkReaddir", func(t *testing.T) {
		dir := mount(t, writable.Config{})

		// Create a regular file and a symlink in the same directory.
		WriteFileOrFail(t, 100, filepath.Join(dir, "regular"))
		require.NoError(t, os.Symlink("/some/target", filepath.Join(dir, "mylink")))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)

		found := false
		for _, e := range entries {
			if e.Name() == "mylink" {
				require.Equal(t, os.ModeSymlink, e.Type()&os.ModeSymlink,
					"readdir should report symlink type for mylink")
				found = true
			}
			if e.Name() == "regular" {
				require.Equal(t, os.FileMode(0), e.Type()&os.ModeSymlink,
					"readdir should not report symlink type for regular file")
			}
		}
		require.True(t, found, "symlink entry not found in readdir")
	})

	t.Run("SymlinkSetattr", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMtime: true})
		link := filepath.Join(dir, "mtimelink")
		require.NoError(t, os.Symlink("/some/target", link))

		mtime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, Lchtimes(link, mtime))

		var stat unix.Stat_t
		require.NoError(t, unix.Lstat(link, &stat))
		gotMtime := time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
		require.WithinDuration(t, mtime, gotMtime, time.Second)
	})

	t.Run("FileSizeReporting", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "sizecheck")
		data := WriteFileOrFail(t, 5555, path)

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, int64(len(data)), info.Size())
	})

	t.Run("FileAttributes", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "attrcheck")
		WriteFileOrFail(t, 100, path)

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.False(t, info.IsDir())
		require.Equal(t, "attrcheck", info.Name())
		require.Equal(t, int64(100), info.Size())
	})

	t.Run("DefaultDirMode", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		sub := filepath.Join(dir, "modedir")
		require.NoError(t, os.Mkdir(sub, 0o755))

		info, err := os.Stat(sub)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})

	// StoreMtime tests.
	t.Run("StoreMtime/disabled", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMtime: false})
		path := filepath.Join(dir, "nomtime")
		WriteFileOrFail(t, 100, path)

		// Without StoreMtime, Getattr returns mtime=0 which the
		// kernel reports as Unix epoch start.
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, time.Unix(0, 0), info.ModTime())
	})

	t.Run("StoreMtime/enabled", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMtime: true})
		path := filepath.Join(dir, "withmtime")
		WriteFileOrFail(t, 100, path)

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.False(t, info.ModTime().IsZero(), "mtime should be set when StoreMtime is on")
		require.WithinDuration(t, time.Now(), info.ModTime(), 30*time.Second)
	})

	// StoreMode tests.
	t.Run("StoreMode/disabled", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMode: false})
		path := filepath.Join(dir, "nomode")
		WriteFileOrFail(t, 100, path)
		// chmod should not fail, even when not persisting
		require.NoError(t, os.Chmod(path, 0o600))

		info, err := os.Stat(path)
		require.NoError(t, err)
		// With StoreMode off, mode stays at default 0644.
		require.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	})

	t.Run("StoreMode/enabled", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMode: true})
		path := filepath.Join(dir, "withmode")
		WriteFileOrFail(t, 100, path)
		require.NoError(t, os.Chmod(path, 0o600))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("SetuidBitsStripped", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMode: true})
		path := filepath.Join(dir, "setuid")
		WriteFileOrFail(t, 100, path)

		// Setuid, setgid, and sticky bits should be silently stripped
		// because boxo's MFS exposes only the lower 9 permission bits.
		require.NoError(t, os.Chmod(path, 0o4755))
		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
	})

	t.Run("DirMtime", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMtime: true})
		sub := filepath.Join(dir, "dirmtime")
		require.NoError(t, os.Mkdir(sub, 0o755))

		mtime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		require.NoError(t, os.Chtimes(sub, mtime, mtime))

		info, err := os.Stat(sub)
		require.NoError(t, err)
		require.WithinDuration(t, mtime, info.ModTime(), time.Second)
	})

	t.Run("DirChmod", func(t *testing.T) {
		dir := mount(t, writable.Config{StoreMode: true})
		sub := filepath.Join(dir, "dirchmod")
		require.NoError(t, os.Mkdir(sub, 0o755))
		require.NoError(t, os.Chmod(sub, 0o700))

		info, err := os.Stat(sub)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	})

	t.Run("XattrCID", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "xattrfile")
		WriteFileOrFail(t, 100, path)

		buf := make([]byte, 256)
		n, err := unix.Getxattr(path, "ipfs.cid", buf)
		require.NoError(t, err)
		require.NotEmpty(t, string(buf[:n]))
	})

	t.Run("UnknownXattr", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "xattrunk")
		WriteFileOrFail(t, 50, path)

		buf := make([]byte, 256)
		_, err := unix.Getxattr(path, "user.nonexistent", buf)
		require.Error(t, err)
	})

	t.Run("ConcurrentWrites", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		nactors := 4
		filesPerActor := 400
		fileSize := 2000

		if racedet.WithRace() {
			nactors = 2
			filesPerActor = 50
		}

		data := make([][][]byte, nactors)
		var wg sync.WaitGroup
		for i := range nactors {
			data[i] = make([][]byte, filesPerActor)
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				for j := range filesPerActor {
					out, err := WriteFile(fileSize, filepath.Join(dir, fmt.Sprintf("%dFILE%d", n, j)))
					if err != nil {
						t.Error(err)
						continue
					}
					data[n][j] = out
				}
			}(i)
		}
		wg.Wait()

		for i := range nactors {
			for j := range filesPerActor {
				if data[i][j] == nil {
					continue
				}
				VerifyFile(t, filepath.Join(dir, fmt.Sprintf("%dFILE%d", i, j)), data[i][j])
			}
		}
	})

	t.Run("ConcurrentRW", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		nfiles := 5
		readers := 5

		content := make([][]byte, nfiles)
		for i := range content {
			content[i] = RandBytes(8196)
		}

		// Write phase.
		var wg sync.WaitGroup
		for i := range nfiles {
			wg.Go(func() {
				if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(i)), content[i], 0o644); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()

		// Read phase.
		for i := range nfiles * readers {
			wg.Go(func() {
				got, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(i/readers)))
				if err != nil {
					t.Error(err)
					return
				}
				if !bytes.Equal(content[i/readers], got) {
					t.Error("read and write not equal")
				}
			})
		}
		wg.Wait()
	})

	// Large file concurrent reads: the kernel sends multiple Read
	// requests via readahead on files bigger than max_read (128 KB).
	// Without proper mutex serialization on the file handle, concurrent
	// reads corrupt the DagReader's internal state.
	t.Run("LargeFileConcurrentRead", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "largeconcurrent")

		size := 1024*1024 + 1 // 1 MiB + 1 byte
		data := WriteFileOrFail(t, size, path)

		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Errorf("ReadFile: %v", err)
					return
				}
				if !bytes.Equal(got, data) {
					t.Errorf("data mismatch: got %d bytes, want %d", len(got), len(data))
				}
			})
		}
		wg.Wait()
	})

	// Simulate the rsync --inplace pattern: one goroutine holds a
	// file open for reading while another opens it for writing.
	// MFS's desclock blocks a write-open while a read descriptor
	// exists. The FUSE layer avoids this by creating a DagReader
	// for read-only opens instead of going through MFS.
	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		path := filepath.Join(dir, "concurrent_rw")

		data := WriteFileOrFail(t, 50000, path)

		// Hold the file open for reading (like rsync's generator).
		reader, err := os.Open(path)
		require.NoError(t, err)
		defer reader.Close()

		// Overwrite the file while the reader is still open
		// (like rsync's receiver).
		newData := RandBytes(60000)
		require.NoError(t, os.WriteFile(path, newData, 0o644))

		// The reader should still see the original snapshot.
		got, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.True(t, bytes.Equal(data, got), "reader should see original data")

		// A fresh read should see the new data.
		got2, err := os.ReadFile(path)
		require.NoError(t, err)
		require.True(t, bytes.Equal(newData, got2), "new reader should see updated data")
	})

	t.Run("FSThrash", func(t *testing.T) {
		dir := mount(t, writable.Config{})
		dirs := []string{dir}
		dirlock := sync.RWMutex{}
		filelock := sync.Mutex{}
		files := make(map[string][]byte)

		ndirWorkers := 2
		nfileWorkers := 2
		ndirs := 100
		nfiles := 200

		var wg sync.WaitGroup

		for i := range ndirWorkers {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				for j := range ndirs {
					dirlock.RLock()
					n := mrand.Intn(len(dirs))
					d := dirs[n]
					dirlock.RUnlock()

					newDir := fmt.Sprintf("%s/dir%d-%d", d, worker, j)
					if err := os.Mkdir(newDir, os.ModeDir); err != nil {
						t.Error(err)
						continue
					}
					dirlock.Lock()
					dirs = append(dirs, newDir)
					dirlock.Unlock()
				}
			}(i)
		}

		for i := range nfileWorkers {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				for j := range nfiles {
					dirlock.RLock()
					n := mrand.Intn(len(dirs))
					d := dirs[n]
					dirlock.RUnlock()

					name := fmt.Sprintf("%s/file%d-%d", d, worker, j)
					data, err := WriteFile(2000+mrand.Intn(5000), name)
					if err != nil {
						t.Error(err)
						continue
					}
					filelock.Lock()
					files[name] = data
					filelock.Unlock()
				}
			}(i)
		}

		wg.Wait()
		for name, data := range files {
			got, err := os.ReadFile(name)
			if err != nil {
				t.Errorf("reading %s: %v", name, err)
				continue
			}
			if !bytes.Equal(data, got) {
				t.Errorf("data mismatch in %s", name)
			}
		}
	})
}

// Test helpers exported for use by mount-specific tests.

// RandBytes returns size random bytes.
func RandBytes(size int) []byte {
	b := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}
	return b
}

// WriteFile writes size random bytes to path and returns the data.
func WriteFile(size int, path string) ([]byte, error) {
	data := RandBytes(size)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, err
	}
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		return nil, err
	}
	// Go's goroutine preemption (SIGURG) can interrupt the FUSE FLUSH
	// inside close(), returning EINTR. This is not data loss: the write
	// already succeeded and the kernel will still send RELEASE.
	if err := f.Close(); err != nil && !errors.Is(err, syscall.EINTR) {
		return nil, err
	}
	return data, nil
}

// WriteFileOrFail calls WriteFile and fails the test on error.
func WriteFileOrFail(t *testing.T, size int, path string) []byte {
	t.Helper()
	data, err := WriteFile(size, path)
	require.NoError(t, err)
	return data
}

// VerifyFile reads the file at path and asserts its contents match want.
func VerifyFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, len(want), len(got), "file size mismatch")
	require.True(t, bytes.Equal(want, got), "file content mismatch")
}

// CheckExists asserts that path exists.
func CheckExists(t *testing.T, path string) {
	t.Helper()
	_, err := os.Stat(path)
	require.NoError(t, err)
}

// Lchtimes sets mtime on a symlink without following it (lutimes).
// Go's os package has no Lchtimes, so we call utimensat directly.
func Lchtimes(path string, mtime time.Time) error {
	ts := unix.NsecToTimespec(mtime.UnixNano())
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, []unix.Timespec{ts, ts}, unix.AT_SYMLINK_NOFOLLOW)
}
