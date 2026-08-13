package commands

import (
	"os"
	"slices"
	"testing"
	"time"

	unixfs_pb "github.com/ipfs/boxo/ipld/unixfs/pb"
	"github.com/stretchr/testify/assert"
)

func TestFormatMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     os.FileMode
		expected string
	}{
		// File types
		{
			name:     "regular file with rw-r--r--",
			mode:     0644,
			expected: "-rw-r--r--",
		},
		{
			name:     "regular file with rwxr-xr-x",
			mode:     0755,
			expected: "-rwxr-xr-x",
		},
		{
			name:     "regular file with no permissions",
			mode:     0,
			expected: "----------",
		},
		{
			name:     "regular file with full permissions",
			mode:     0777,
			expected: "-rwxrwxrwx",
		},
		{
			name:     "directory with rwxr-xr-x",
			mode:     os.ModeDir | 0755,
			expected: "drwxr-xr-x",
		},
		{
			name:     "directory with rwx------",
			mode:     os.ModeDir | 0700,
			expected: "drwx------",
		},
		{
			name:     "symlink with rwxrwxrwx",
			mode:     os.ModeSymlink | 0777,
			expected: "lrwxrwxrwx",
		},
		{
			name:     "named pipe with rw-r--r--",
			mode:     os.ModeNamedPipe | 0644,
			expected: "prw-r--r--",
		},
		{
			name:     "socket with rw-rw-rw-",
			mode:     os.ModeSocket | 0666,
			expected: "srw-rw-rw-",
		},
		{
			name:     "block device with rw-rw----",
			mode:     os.ModeDevice | 0660,
			expected: "brw-rw----",
		},
		{
			name:     "character device with rw-rw-rw-",
			mode:     os.ModeDevice | os.ModeCharDevice | 0666,
			expected: "crw-rw-rw-",
		},

		// Special permission bits - setuid
		{
			name:     "setuid with execute",
			mode:     os.ModeSetuid | 0755,
			expected: "-rwsr-xr-x",
		},
		{
			name:     "setuid without execute",
			mode:     os.ModeSetuid | 0644,
			expected: "-rwSr--r--",
		},

		// Special permission bits - setgid
		{
			name:     "setgid with execute",
			mode:     os.ModeSetgid | 0755,
			expected: "-rwxr-sr-x",
		},
		{
			name:     "setgid without execute",
			mode:     os.ModeSetgid | 0745,
			expected: "-rwxr-Sr-x",
		},

		// Special permission bits - sticky
		{
			name:     "sticky with execute",
			mode:     os.ModeSticky | 0755,
			expected: "-rwxr-xr-t",
		},
		{
			name:     "sticky without execute",
			mode:     os.ModeSticky | 0754,
			expected: "-rwxr-xr-T",
		},

		// Combined special bits
		{
			name:     "setuid + setgid + sticky all with execute",
			mode:     os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0777,
			expected: "-rwsrwsrwt",
		},
		{
			name:     "setuid + setgid + sticky none with execute",
			mode:     os.ModeSetuid | os.ModeSetgid | os.ModeSticky | 0666,
			expected: "-rwSrwSrwT",
		},

		// Directory with special bits
		{
			name:     "directory with sticky bit",
			mode:     os.ModeDir | os.ModeSticky | 0755,
			expected: "drwxr-xr-t",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := formatMode(tc.mode)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestFormatModTime(t *testing.T) {
	t.Parallel()

	t.Run("zero time returns dash", func(t *testing.T) {
		t.Parallel()
		result := formatModTime(time.Time{})
		assert.Equal(t, "-", result)
	})

	t.Run("old time shows year format", func(t *testing.T) {
		t.Parallel()
		// Use a time clearly in the past (more than 6 months ago)
		oldTime := time.Date(2020, time.March, 15, 10, 30, 0, 0, time.UTC)
		result := formatModTime(oldTime)
		// Format: "Jan 02  2006" (note: two spaces before year)
		assert.Equal(t, "Mar 15  2020", result)
	})

	t.Run("very old time shows year format", func(t *testing.T) {
		t.Parallel()
		veryOldTime := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
		result := formatModTime(veryOldTime)
		assert.Equal(t, "Jan 01  2000", result)
	})

	t.Run("future time shows year format", func(t *testing.T) {
		t.Parallel()
		// Times more than 24h in the future should show year format
		futureTime := time.Now().AddDate(1, 0, 0)
		result := formatModTime(futureTime)
		// Should contain the future year
		assert.Contains(t, result, "  ")                         // two spaces before year
		assert.Regexp(t, `^[A-Z][a-z]{2} \d{2}  \d{4}$`, result) // matches "Mon DD  YYYY"
		assert.Contains(t, result, futureTime.Format("2006"))    // contains the year
	})

	t.Run("format lengths are consistent", func(t *testing.T) {
		t.Parallel()
		// Both formats should produce 12-character strings for alignment
		oldTime := time.Date(2020, time.March, 15, 10, 30, 0, 0, time.UTC)
		oldResult := formatModTime(oldTime)
		assert.Len(t, oldResult, 12, "old time format should be 12 chars")

		// Recent time: use 1 month ago to ensure it's always within the 6-month window
		recentTime := time.Now().AddDate(0, -1, 0)
		recentResult := formatModTime(recentTime)
		assert.Len(t, recentResult, 12, "recent time format should be 12 chars")
	})
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size   uint64
		human  bool
		expect string
	}{
		{0, false, "0"},
		{0, true, "0 B"},
		{1, false, "1"},
		{1, true, "1 B"},
		{999, true, "999 B"},
		{1000, true, "1.0 kB"},
		{1500, true, "1.5 kB"},
		{1000000, true, "1.0 MB"},
		{1234567, true, "1.2 MB"},
		{1000000000, true, "1.0 GB"},
		{1234567890, false, "1234567890"},
		{1234567890, true, "1.2 GB"},
		{1000000000000, true, "1.0 TB"},
	}

	for _, tt := range tests {
		got := formatSize(tt.size, tt.human)
		assert.Equal(t, tt.expect, got, "formatSize(%d, %v)", tt.size, tt.human)
	}
}

// lsLinkNames returns entry names in slice order, so a failed sort assertion
// reports the actual order instead of a wall of LsLink structs.
func lsLinkNames(links []LsLink) []string {
	names := make([]string, len(links))
	for i, link := range links {
		names[i] = link.Name
	}
	return names
}

func TestLsLinkByName(t *testing.T) {
	t.Parallel()

	links := []LsLink{
		{Name: "z.bin", Size: 100},
		{Name: "a.bin", Size: 200},
		{Name: "m.bin", Size: 100},
	}
	slices.SortFunc(links, lsLinkByName)
	assert.Equal(t, []string{"a.bin", "m.bin", "z.bin"}, lsLinkNames(links))
}

func TestLsLinkBySize(t *testing.T) {
	t.Parallel()

	t.Run("largest first", func(t *testing.T) {
		t.Parallel()
		// Names are deliberately out of step with sizes. With names that happen
		// to already be in size order, this would pass under lsLinkByName too.
		links := []LsLink{
			{Name: "a-small.bin", Size: 100},
			{Name: "m-large.bin", Size: 100000},
			{Name: "z-medium.bin", Size: 500},
		}
		slices.SortFunc(links, lsLinkBySize)
		assert.Equal(t, []string{"m-large.bin", "z-medium.bin", "a-small.bin"}, lsLinkNames(links))
	})

	t.Run("equal sizes fall back to name", func(t *testing.T) {
		t.Parallel()
		links := []LsLink{
			{Name: "b.bin", Size: 100},
			{Name: "a.bin", Size: 100},
		}
		slices.SortFunc(links, lsLinkBySize)
		assert.Equal(t, []string{"a.bin", "b.bin"}, lsLinkNames(links))
	})

	t.Run("directories rank below any non-empty file", func(t *testing.T) {
		t.Parallel()
		// UnixFS directories carry no Filesize, so they arrive here as 0 no
		// matter how much content they hold. They do not sort strictly last
		// though: at 0 they tie with empty files and break by name.
		links := []LsLink{
			{Name: "subdir", Size: 0, Type: unixfs_pb.Data_Directory},
			{Name: "tiny.bin", Size: 1, Type: unixfs_pb.Data_File},
		}
		slices.SortFunc(links, lsLinkBySize)
		assert.Equal(t, []string{"tiny.bin", "subdir"}, lsLinkNames(links))
	})
}
