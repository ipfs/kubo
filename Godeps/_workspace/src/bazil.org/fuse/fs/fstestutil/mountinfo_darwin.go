package fstestutil

import (
	"regexp"
	"syscall"
)

// cstr converts a nil-terminated C string into a Go string
func cstr(ca []int8) string {
	s := make([]byte, 0, len(ca))
	for _, c := range ca {
		if c == 0x00 {
			break
		}
		s = append(s, byte(c))
	}
	return string(s)
}

var re = regexp.MustCompile(`\\(.)`)

// unescape removes backslash-escaping. The escaped characters are not
// mapped in any way; that is, unescape(`\n` ) == `n`.
func unescape(s string) string {
	return re.ReplaceAllString(s, `$1`)
}

func getMountInfo(mnt string) (*MountInfo, error) {
	var st syscall.Statfs_t
	err := syscall.Statfs(mnt, &st)
	if err != nil {
		return nil, err
	}
	i := &MountInfo{
		// osx getmntent(3) fails to un-escape the data, so we do it..
		// this might lead to double-unescaping in the future. fun.
		// TestMountOptionFSNameEvilBackslashDouble checks for that.
		FSName: unescape(cstr(st.Mntfromname[:])),
	}
	return i, nil
}
