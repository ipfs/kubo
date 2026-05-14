package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

func TestPinNameValidation(t *testing.T) {
	t.Parallel()

	// Create a test node and add a test file
	node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
	defer node.StopDaemon()

	// Add a test file to get a CID
	testContent := "test content for pin name validation"
	testCID := node.IPFSAddStr(testContent, "--pin=false")

	t.Run("pin add accepts valid names", func(t *testing.T) {
		testCases := []struct {
			name        string
			pinName     string
			description string
		}{
			{
				name:        "empty_name",
				pinName:     "",
				description: "Empty name should be allowed",
			},
			{
				name:        "short_name",
				pinName:     "test",
				description: "Short ASCII name should be allowed",
			},
			{
				name:        "max_255_bytes",
				pinName:     strings.Repeat("a", 255),
				description: "Exactly 255 bytes should be allowed",
			},
			{
				name:        "unicode_within_limit",
				pinName:     "测试名称🔥", // Chinese characters and emoji
				description: "Unicode characters within 255 bytes should be allowed",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				var args []string
				if tc.pinName != "" {
					args = []string{"pin", "add", "--name", tc.pinName, testCID}
				} else {
					args = []string{"pin", "add", testCID}
				}

				res := node.RunIPFS(args...)
				require.Equal(t, 0, res.ExitCode(), tc.description)

				// Clean up - unpin
				node.RunIPFS("pin", "rm", testCID)
			})
		}
	})

	t.Run("pin add rejects names exceeding 255 bytes", func(t *testing.T) {
		testCases := []struct {
			name        string
			pinName     string
			description string
		}{
			{
				name:        "256_bytes",
				pinName:     strings.Repeat("a", 256),
				description: "256 bytes should be rejected",
			},
			{
				name:        "300_bytes",
				pinName:     strings.Repeat("b", 300),
				description: "300 bytes should be rejected",
			},
			{
				name:        "unicode_exceeding_limit",
				pinName:     strings.Repeat("测", 100), // Each Chinese character is 3 bytes, total 300 bytes
				description: "Unicode string exceeding 255 bytes should be rejected",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res := node.RunIPFS("pin", "add", "--name", tc.pinName, testCID)
				require.NotEqual(t, 0, res.ExitCode(), tc.description)
				require.Contains(t, res.Stderr.String(), "max 255 bytes", "Error should mention the 255 byte limit")
			})
		}
	})

	t.Run("pin ls with name filter validates length", func(t *testing.T) {
		// Test valid filter
		res := node.RunIPFS("pin", "ls", "--name", strings.Repeat("a", 255))
		require.Equal(t, 0, res.ExitCode(), "255-byte name filter should be accepted")

		// Test invalid filter
		res = node.RunIPFS("pin", "ls", "--name", strings.Repeat("a", 256))
		require.NotEqual(t, 0, res.ExitCode(), "256-byte name filter should be rejected")
		require.Contains(t, res.Stderr.String(), "max 255 bytes", "Error should mention the 255 byte limit")
	})
}

func TestAddPinNameValidation(t *testing.T) {
	t.Parallel()

	node := harness.NewT(t).NewNode().Init().StartDaemon("--offline")
	defer node.StopDaemon()

	// Create a test file
	testFile := "test.txt"
	node.WriteBytes(testFile, []byte("test content for add command"))

	t.Run("ipfs add with --pin-name accepts valid names", func(t *testing.T) {
		testCases := []struct {
			name        string
			pinName     string
			description string
		}{
			{
				name:        "short_name",
				pinName:     "test-add",
				description: "Short ASCII name should be allowed",
			},
			{
				name:        "max_255_bytes",
				pinName:     strings.Repeat("x", 255),
				description: "Exactly 255 bytes should be allowed",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res := node.RunIPFS("add", fmt.Sprintf("--pin-name=%s", tc.pinName), "-q", testFile)
				require.Equal(t, 0, res.ExitCode(), tc.description)
				cid := strings.TrimSpace(res.Stdout.String())

				// Verify pin exists with name
				lsRes := node.RunIPFS("pin", "ls", "--names", "--type=recursive", cid)
				require.Equal(t, 0, lsRes.ExitCode())
				require.Contains(t, lsRes.Stdout.String(), tc.pinName, "Pin should have the specified name")

				// Clean up
				node.RunIPFS("pin", "rm", cid)
			})
		}
	})

	t.Run("ipfs add with --pin-name rejects names exceeding 255 bytes", func(t *testing.T) {
		testCases := []struct {
			name        string
			pinName     string
			description string
		}{
			{
				name:        "256_bytes",
				pinName:     strings.Repeat("y", 256),
				description: "256 bytes should be rejected",
			},
			{
				name:        "500_bytes",
				pinName:     strings.Repeat("z", 500),
				description: "500 bytes should be rejected",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				res := node.RunIPFS("add", fmt.Sprintf("--pin-name=%s", tc.pinName), testFile)
				require.NotEqual(t, 0, res.ExitCode(), tc.description)
				require.Contains(t, res.Stderr.String(), "max 255 bytes", "Error should mention the 255 byte limit")
			})
		}
	})
}
