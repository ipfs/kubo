package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestGetPunctuationPaths is regression coverage for
// https://github.com/ipfs/kubo/issues/9369, where "ipfs get <cid>/<segment>"
// failed when a path segment was "]". Every segment below is a valid UnixFS
// link name and a valid filename on Linux, macOS, and Windows, so retrieval
// must work for each of them. Several are also sensitive to a POSIX shell;
// driving ipfs directly here (rather than through sharness) sidesteps shell
// quoting, so even the apostrophe case is covered. Each case is exercised both
// offline and against a running daemon, since the path is parsed client-side
// in both modes.
func TestGetPunctuationPaths(t *testing.T) {
	t.Parallel()

	// Subtest names are plain identifiers, not the raw segment, so that
	// `go test -run` can target a single case (a bare "[" or "(" is an
	// invalid -run regexp).
	cases := []struct {
		name    string
		segment string
	}{
		{"open-bracket", "["},
		{"close-bracket", "]"},
		{"open-brace", "{"},
		{"close-brace", "}"},
		{"open-paren", "("},
		{"close-paren", ")"},
		{"space", "space name"},
		{"hash", "hash#name"},
		{"percent", "percent%name"},
		{"comma", "comma,name"},
		{"plus", "plus+name"},
		{"equals", "equals=name"},
		{"at", "at@name"},
		{"dollar", "dollar$name"},
		{"ampersand", "ampersand&name"},
		{"semicolon", "semicolon;name"},
		{"apostrophe", "apostrophe'name"},
		{"tilde", "tilde~name"},
	}

	// Build a directory holding one file per segment, each with distinct
	// content so a mixed-up retrieval is caught, then retrieve every file by
	// its "<cid>/<segment>" path and confirm the bytes round-trip.
	retrieveEach := func(t *testing.T, node *harness.Node) {
		srcDir := t.TempDir()
		for _, tc := range cases {
			content := "content for " + tc.name + "\n"
			require.NoError(t, os.WriteFile(filepath.Join(srcDir, tc.segment), []byte(content), 0o644))
		}

		cid := node.IPFS("add", "-r", "-Q", srcDir).Stdout.Trimmed()

		outDir := t.TempDir()
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				out := filepath.Join(outDir, tc.name)
				node.IPFS("get", "-o", out, cid+"/"+tc.segment)
				got, err := os.ReadFile(out)
				require.NoError(t, err)
				require.Equal(t, "content for "+tc.name+"\n", string(got))
			})
		}
	}

	t.Run("offline", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		retrieveEach(t, node)
	})

	t.Run("online", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		defer node.StopDaemon()
		retrieveEach(t, node)
	})
}
