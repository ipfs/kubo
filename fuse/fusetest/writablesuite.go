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
