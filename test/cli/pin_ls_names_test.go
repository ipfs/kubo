package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// pinInfo represents the JSON structure for pin ls output
type pinInfo struct {
	Type string `json:"Type"`
	Name string `json:"Name"`
}

// pinLsJSON represents the JSON output structure for pin ls command
type pinLsJSON struct {
	Keys map[string]pinInfo `json:"Keys"`
}

// Helper function to initialize a test node with daemon
func setupTestNode(t *testing.T) *harness.Node {
	t.Helper()
	node := harness.NewT(t).NewNode().Init()
	node.StartDaemon("--offline")
	return node
}

// Helper function to assert pin name and CID are present in output
func assertPinOutput(t *testing.T, output, cid, pinName string) {
	t.Helper()
	require.Contains(t, output, pinName, "pin name '%s' not found in output: %s", pinName, output)
	require.Contains(t, output, cid, "CID %s not found in output: %s", cid, output)
}

// Helper function to assert CID is present but name is not
func assertCIDOnly(t *testing.T, output, cid string) {
	t.Helper()
	require.Contains(t, output, cid, "CID %s not found in output: %s", cid, output)
}

// Helper function to assert neither CID nor name are present
func assertNotPresent(t *testing.T, output, cid, pinName string) {
	t.Helper()
	require.NotContains(t, output, cid, "CID %s should not be present in output: %s", cid, output)
	require.NotContains(t, output, pinName, "pin name '%s' should not be present in output: %s", pinName, output)
}

// Test that pin ls returns names when querying specific CIDs with --names flag
func TestPinLsWithNamesForSpecificCIDs(t *testing.T) {
	t.Parallel()

	t.Run("pin ls with specific CID returns name", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add content without pinning
		cidA := node.IPFSAddStr("content A", "--pin=false")
		cidB := node.IPFSAddStr("content B", "--pin=false")
		cidC := node.IPFSAddStr("content C", "--pin=false")

		// Pin with names
		node.IPFS("pin", "add", "--name=pin-a", cidA)
		node.IPFS("pin", "add", "--name=pin-b", cidB)
		node.IPFS("pin", "add", cidC) // No name

		// Test: pin ls <cid> --names should return the name
		res := node.IPFS("pin", "ls", cidA, "--names")
		assertPinOutput(t, res.Stdout.String(), cidA, "pin-a")

		res = node.IPFS("pin", "ls", cidB, "--names")
		assertPinOutput(t, res.Stdout.String(), cidB, "pin-b")

		// Test: pin without name should work
		res = node.IPFS("pin", "ls", cidC, "--names")
		output := res.Stdout.String()
		assertCIDOnly(t, output, cidC)
		require.Contains(t, output, "recursive", "pin type 'recursive' not found for CID %s in output: %s", cidC, output)

		// Test: without --names flag, no names returned
		res = node.IPFS("pin", "ls", cidA)
		output = res.Stdout.String()
		require.NotContains(t, output, "pin-a", "pin name 'pin-a' should not be present without --names flag, but found in: %s", output)
		assertCIDOnly(t, output, cidA)
	})

	t.Run("pin ls with multiple CIDs returns names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create test content
		cidA := node.IPFSAddStr("multi A", "--pin=false")
		cidB := node.IPFSAddStr("multi B", "--pin=false")

		// Pin with names
		node.IPFS("pin", "add", "--name=multi-pin-a", cidA)
		node.IPFS("pin", "add", "--name=multi-pin-b", cidB)

		// Test multiple CIDs at once
		res := node.IPFS("pin", "ls", cidA, cidB, "--names")
		output := res.Stdout.String()
		assertPinOutput(t, output, cidA, "multi-pin-a")
		assertPinOutput(t, output, cidB, "multi-pin-b")
	})

	t.Run("pin ls without CID lists all pins with names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create and pin content with names
		cidA := node.IPFSAddStr("list all A", "--pin=false")
		cidB := node.IPFSAddStr("list all B", "--pin=false")
		cidC := node.IPFSAddStr("list all C", "--pin=false")

		node.IPFS("pin", "add", "--name=all-pin-a", cidA)
		node.IPFS("pin", "add", "--name=all-pin-b", "--recursive=false", cidB)
		node.IPFS("pin", "add", cidC) // No name

		// Test: pin ls --names (without CID) should list all pins with their names
		res := node.IPFS("pin", "ls", "--names")
		output := res.Stdout.String()

		// Should contain all pins with their names
		assertPinOutput(t, output, cidA, "all-pin-a")
		assertPinOutput(t, output, cidB, "all-pin-b")
		assertCIDOnly(t, output, cidC)

		// Pin C should appear but without a name (just type)
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, cidC) {
				// Should have CID and type but no name
				require.Contains(t, line, "recursive", "pin type 'recursive' not found for unnamed pin %s in line: %s", cidC, line)
				require.NotContains(t, line, "all-pin", "pin name should not be present for unnamed pin %s, but found in line: %s", cidC, line)
			}
		}
	})

	t.Run("pin ls --type with --names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create test content
		cidDirect := node.IPFSAddStr("direct content", "--pin=false")
		cidRecursive := node.IPFSAddStr("recursive content", "--pin=false")

		// Create a DAG for indirect testing
		childCid := node.IPFSAddStr("child for indirect", "--pin=false")
		parentContent := fmt.Sprintf(`{"link": "/ipfs/%s"}`, childCid)
		parentCid := node.PipeStrToIPFS(parentContent, "dag", "put", "--input-codec=json", "--store-codec=dag-cbor").Stdout.Trimmed()

		// Pin with different types and names
		node.IPFS("pin", "add", "--name=direct-pin", "--recursive=false", cidDirect)
		node.IPFS("pin", "add", "--name=recursive-pin", cidRecursive)
		node.IPFS("pin", "add", "--name=parent-pin", parentCid)

		// Test: --type=direct --names
		res := node.IPFS("pin", "ls", "--type=direct", "--names")
		output := res.Stdout.String()
		assertPinOutput(t, output, cidDirect, "direct-pin")
		assertNotPresent(t, output, cidRecursive, "recursive-pin")

		// Test: --type=recursive --names
		res = node.IPFS("pin", "ls", "--type=recursive", "--names")
		output = res.Stdout.String()
		assertPinOutput(t, output, cidRecursive, "recursive-pin")
		assertPinOutput(t, output, parentCid, "parent-pin")
		assertNotPresent(t, output, cidDirect, "direct-pin")

		// Test: --type=indirect with proper directory structure
		// Create a directory with a file for indirect pin testing
		dirPath := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("test content"), 0644))

		// Add directory recursively
		dirAddRes := node.IPFS("add", "-r", "-q", dirPath)
		dirCidStr := strings.TrimSpace(dirAddRes.Stdout.Lines()[len(dirAddRes.Stdout.Lines())-1])

		// Add file separately without pinning to get its CID
		fileAddRes := node.IPFS("add", "-q", "--pin=false", filepath.Join(dirPath, "file.txt"))
		fileCidStr := strings.TrimSpace(fileAddRes.Stdout.String())

		// Check if file shows as indirect
		res = node.IPFS("pin", "ls", "--type=indirect", fileCidStr)
		output = res.Stdout.String()
		require.Contains(t, output, fileCidStr, "indirect pin CID %s not found in output: %s", fileCidStr, output)
		require.Contains(t, output, "indirect through "+dirCidStr, "indirect relationship not found for CID %s through %s in output: %s", fileCidStr, dirCidStr, output)

		// Test: --type=all --names
		res = node.IPFS("pin", "ls", "--type=all", "--names")
		output = res.Stdout.String()
		assertPinOutput(t, output, cidDirect, "direct-pin")
		assertPinOutput(t, output, cidRecursive, "recursive-pin")
		assertPinOutput(t, output, parentCid, "parent-pin")
		// Indirect pins are included in --type=all output
	})

	t.Run("pin ls JSON output with names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add and pin content with name
		cidA := node.IPFSAddStr("json content", "--pin=false")
		node.IPFS("pin", "add", "--name=json-pin", cidA)

		// Test JSON output with specific CID
		res := node.IPFS("pin", "ls", cidA, "--names", "--enc=json")
		var pinOutput pinLsJSON
		err := json.Unmarshal([]byte(res.Stdout.String()), &pinOutput)
		require.NoError(t, err, "failed to unmarshal JSON output: %s", res.Stdout.String())

		pinData, ok := pinOutput.Keys[cidA]
		require.True(t, ok, "CID %s should be in Keys map, got: %+v", cidA, pinOutput.Keys)
		require.Equal(t, "recursive", pinData.Type, "expected pin type 'recursive', got '%s'", pinData.Type)
		require.Equal(t, "json-pin", pinData.Name, "expected pin name 'json-pin', got '%s'", pinData.Name)

		// Without names flag
		res = node.IPFS("pin", "ls", cidA, "--enc=json")
		err = json.Unmarshal([]byte(res.Stdout.String()), &pinOutput)
		require.NoError(t, err, "failed to unmarshal JSON output: %s", res.Stdout.String())

		pinData, ok = pinOutput.Keys[cidA]
		require.True(t, ok, "CID %s should be in Keys map, got: %+v", cidA, pinOutput.Keys)
		// Name should be empty without --names flag
		require.Equal(t, "", pinData.Name, "pin name should be empty without --names flag, got '%s'", pinData.Name)

		// Test JSON output without CID (list all)
		res = node.IPFS("pin", "ls", "--names", "--enc=json")
		var listOutput pinLsJSON
		err = json.Unmarshal([]byte(res.Stdout.String()), &listOutput)
		require.NoError(t, err, "failed to unmarshal JSON list output: %s", res.Stdout.String())
		// Should have at least one pin (the one we just added)
		require.NotEmpty(t, listOutput.Keys, "pin list should not be empty")
		// Check that our pin is in the list
		pinData, ok = listOutput.Keys[cidA]
		require.True(t, ok, "our pin with CID %s should be in the list, got: %+v", cidA, listOutput.Keys)
		require.Equal(t, "json-pin", pinData.Name, "expected pin name 'json-pin' in list, got '%s'", pinData.Name)
	})

	t.Run("direct and indirect pins with names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create a small DAG: parent -> child
		childCid := node.IPFSAddStr("child content", "--pin=false")

		// Create parent that references child
		parentContent := fmt.Sprintf(`{"link": "/ipfs/%s"}`, childCid)
		parentCid := node.PipeStrToIPFS(parentContent, "dag", "put", "--input-codec=json", "--store-codec=dag-cbor").Stdout.Trimmed()

		// Pin child directly with a name
		node.IPFS("pin", "add", "--name=direct-child", "--recursive=false", childCid)

		// Pin parent recursively with a name
		node.IPFS("pin", "add", "--name=recursive-parent", parentCid)

		// Check direct pin with specific CID
		res := node.IPFS("pin", "ls", "--type=direct", childCid, "--names")
		output := res.Stdout.String()
		require.Contains(t, output, "direct-child", "pin name 'direct-child' not found in output: %s", output)
		require.Contains(t, output, "direct", "pin type 'direct' not found in output: %s", output)

		// Check recursive pin with specific CID
		res = node.IPFS("pin", "ls", "--type=recursive", parentCid, "--names")
		output = res.Stdout.String()
		require.Contains(t, output, "recursive-parent", "pin name 'recursive-parent' not found in output: %s", output)
		require.Contains(t, output, "recursive", "pin type 'recursive' not found in output: %s", output)

		// Child is both directly pinned and indirectly pinned through parent
		// Both relationships are valid and can be checked
	})

	t.Run("pin update preserves name", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create two pieces of content
		cidOld := node.IPFSAddStr("old content", "--pin=false")
		cidNew := node.IPFSAddStr("new content", "--pin=false")

		// Pin with name
		node.IPFS("pin", "add", "--name=my-pin", cidOld)

		// Update pin
		node.IPFS("pin", "update", cidOld, cidNew)

		// Check that new pin has the same name
		res := node.IPFS("pin", "ls", cidNew, "--names")
		require.Contains(t, res.Stdout.String(), "my-pin", "pin name 'my-pin' not preserved after update, output: %s", res.Stdout.String())

		// Old pin should not exist
		res = node.RunIPFS("pin", "ls", cidOld)
		require.Equal(t, 1, res.ExitCode(), "expected exit code 1 for unpinned CID, got %d", res.ExitCode())
		require.Contains(t, res.Stderr.String(), "is not pinned", "expected 'is not pinned' error for old CID %s, got: %s", cidOld, res.Stderr.String())
	})

	t.Run("pin ls with invalid CID returns error", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()

		res := node.RunIPFS("pin", "ls", "invalid-cid")
		require.Equal(t, 1, res.ExitCode(), "expected exit code 1 for invalid CID, got %d", res.ExitCode())
		require.Contains(t, res.Stderr.String(), "invalid", "expected 'invalid' in error message, got: %s", res.Stderr.String())
	})

	t.Run("pin ls with unpinned CID returns error", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add content without pinning
		cid := node.IPFSAddStr("unpinned content", "--pin=false")

		res := node.RunIPFS("pin", "ls", cid)
		require.Equal(t, 1, res.ExitCode(), "expected exit code 1 for unpinned CID, got %d", res.ExitCode())
		require.Contains(t, res.Stderr.String(), "is not pinned", "expected 'is not pinned' error for CID %s, got: %s", cid, res.Stderr.String())
	})

	t.Run("pin with special characters in name", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		testCases := []struct {
			name    string
			pinName string
		}{
			{"unicode", "test-📌-pin"},
			{"spaces", "test pin name"},
			{"special chars", "test!@#$%"},
			{"path-like", "test/pin/name"},
			{"dots", "test.pin.name"},
			{"long name", strings.Repeat("a", 255)},
			{"empty name", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				cid := node.IPFSAddStr("content for "+tc.name, "--pin=false")
				node.IPFS("pin", "add", "--name="+tc.pinName, cid)

				res := node.IPFS("pin", "ls", cid, "--names")
				if tc.pinName != "" {
					require.Contains(t, res.Stdout.String(), tc.pinName,
						"pin name '%s' not found in output for test case '%s'", tc.pinName, tc.name)
				}
			})
		}
	})

	t.Run("concurrent pin operations with names", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Create multiple goroutines adding pins with names
		numPins := 10
		done := make(chan struct{}, numPins)

		for i := 0; i < numPins; i++ {
			go func(idx int) {
				defer func() { done <- struct{}{} }()

				content := fmt.Sprintf("concurrent content %d", idx)
				cid := node.IPFSAddStr(content, "--pin=false")
				pinName := fmt.Sprintf("concurrent-pin-%d", idx)
				node.IPFS("pin", "add", "--name="+pinName, cid)
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numPins; i++ {
			<-done
		}

		// Verify all pins have correct names
		res := node.IPFS("pin", "ls", "--names")
		output := res.Stdout.String()
		for i := 0; i < numPins; i++ {
			pinName := fmt.Sprintf("concurrent-pin-%d", i)
			require.Contains(t, output, pinName,
				"concurrent pin name '%s' not found in output", pinName)
		}
	})

	t.Run("pin rm removes name association", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add and pin with name
		cid := node.IPFSAddStr("content to remove", "--pin=false")
		node.IPFS("pin", "add", "--name=to-be-removed", cid)

		// Verify pin exists with name
		res := node.IPFS("pin", "ls", cid, "--names")
		require.Contains(t, res.Stdout.String(), "to-be-removed")

		// Remove pin
		node.IPFS("pin", "rm", cid)

		// Verify pin and name are gone
		res = node.RunIPFS("pin", "ls", cid)
		require.Equal(t, 1, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "is not pinned")
	})

	t.Run("garbage collection preserves named pins", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add content with and without pin names
		cidNamed := node.IPFSAddStr("named content", "--pin=false")
		cidUnnamed := node.IPFSAddStr("unnamed content", "--pin=false")
		cidUnpinned := node.IPFSAddStr("unpinned content", "--pin=false")

		node.IPFS("pin", "add", "--name=important-data", cidNamed)
		node.IPFS("pin", "add", cidUnnamed)

		// Run garbage collection
		node.IPFS("repo", "gc")

		// Named and unnamed pins should still exist
		res := node.IPFS("pin", "ls", cidNamed, "--names")
		require.Contains(t, res.Stdout.String(), "important-data")

		res = node.IPFS("pin", "ls", cidUnnamed)
		require.Contains(t, res.Stdout.String(), cidUnnamed)

		// Unpinned content should be gone (cat should fail)
		res = node.RunIPFS("cat", cidUnpinned)
		require.NotEqual(t, 0, res.ExitCode(), "unpinned content should be garbage collected")
	})

	t.Run("pin add with same name can be used for multiple pins", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)

		// Add two different pieces of content
		cid1 := node.IPFSAddStr("first content", "--pin=false")
		cid2 := node.IPFSAddStr("second content", "--pin=false")

		// Pin both with the same name - this is allowed
		node.IPFS("pin", "add", "--name=shared-name", cid1)
		node.IPFS("pin", "add", "--name=shared-name", cid2)

		// List all pins with names
		res := node.IPFS("pin", "ls", "--names")
		output := res.Stdout.String()

		// Both CIDs should be pinned
		require.Contains(t, output, cid1)
		require.Contains(t, output, cid2)

		// Both pins can have the same name
		lines := strings.Split(output, "\n")
		foundCid1WithName := false
		foundCid2WithName := false
		for _, line := range lines {
			if strings.Contains(line, cid1) && strings.Contains(line, "shared-name") {
				foundCid1WithName = true
			}
			if strings.Contains(line, cid2) && strings.Contains(line, "shared-name") {
				foundCid2WithName = true
			}
		}
		require.True(t, foundCid1WithName, "first pin should have the name")
		require.True(t, foundCid2WithName, "second pin should have the name")
	})

	t.Run("pin names persist across daemon restarts", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.StartDaemon("--offline")

		// Add content with pin name
		cid := node.IPFSAddStr("persistent content")
		node.IPFS("pin", "add", "--name=persistent-pin", cid)

		// Restart daemon
		node.StopDaemon()
		node.StartDaemon("--offline")

		// Check pin name persisted
		res := node.IPFS("pin", "ls", cid, "--names")
		require.Contains(t, res.Stdout.String(), "persistent-pin",
			"pin name should persist across daemon restarts")

		node.StopDaemon()
	})
}

// TestPinLsEdgeCases tests edge cases for pin ls command
func TestPinLsEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("invalid pin type returns error", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)
		defer node.StopDaemon()

		// Try to list pins with invalid type
		res := node.RunIPFS("pin", "ls", "--type=invalid")
		require.NotEqual(t, 0, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "invalid type 'invalid'")
		require.Contains(t, res.Stderr.String(), "must be one of {direct, indirect, recursive, all}")
	})

	t.Run("non-existent path returns proper error", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)
		defer node.StopDaemon()

		// Try to list a non-existent CID
		fakeCID := "QmNonExistent123456789"
		res := node.RunIPFS("pin", "ls", fakeCID)
		require.NotEqual(t, 0, res.ExitCode())
	})

	t.Run("unpinned CID returns not pinned error", func(t *testing.T) {
		t.Parallel()
		node := setupTestNode(t)
		defer node.StopDaemon()

		// Add content but don't pin it explicitly (it's just in blockstore)
		unpinnedCID := node.IPFSAddStr("unpinned content", "--pin=false")

		// Try to list specific unpinned CID
		res := node.RunIPFS("pin", "ls", unpinnedCID)
		require.NotEqual(t, 0, res.ExitCode())
		require.Contains(t, res.Stderr.String(), "is not pinned")
	})
}
