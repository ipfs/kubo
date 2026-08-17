//go:build !nofuse

package fusetest

import (
	"bytes"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// assertDirEntryInos checks that the inode number a directory listing reports
// for each entry (d_ino from getdents) is the one stat reports for it. A mount
// fills the two in from separate code paths, and tools take the listing's
// number as authoritative because it costs them no extra syscall.
func assertDirEntryInos(t *testing.T, dir string) {
	t.Helper()

	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	require.NoError(t, err)
	defer unix.Close(fd)

	buf := make([]byte, 8192)
	seen := 0
	for {
		n, err := unix.Getdents(fd, buf)
		require.NoError(t, err)
		if n == 0 {
			break
		}
		for b := buf[:n]; len(b) > 0; {
			ent := (*unix.Dirent)(unsafe.Pointer(&b[0]))
			require.NotZero(t, ent.Reclen, "a dirent of no length would never end the loop")
			b = b[ent.Reclen:]

			name := string(bytes.SplitN(unsafe.Slice((*byte)(unsafe.Pointer(&ent.Name[0])), len(ent.Name)), []byte{0}, 2)[0])
			if name == "." || name == ".." {
				continue
			}

			var st unix.Stat_t
			require.NoError(t, unix.Lstat(filepath.Join(dir, name), &st))
			require.Equal(t, st.Ino, ent.Ino,
				"inode number for %q differs between the directory listing and stat", name)
			seen++
		}
	}
	require.NotZero(t, seen, "expected the listing to report at least one entry")
}
