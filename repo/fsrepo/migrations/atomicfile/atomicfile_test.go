package atomicfile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNew_Success verifies atomic file creation
func TestNew_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)
	defer func() { _ = af.Abort() }()

	// Verify temp file exists
	assert.FileExists(t, af.File.Name())

	// Verify temp file is in same directory
	assert.Equal(t, dir, filepath.Dir(af.File.Name()))
}

// TestClose_Success verifies atomic replacement
func TestClose_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)

	content := []byte("test content")
	_, err = af.Write(content)
	require.NoError(t, err)

	tempName := af.File.Name()

	require.NoError(t, af.Close())

	// Verify target file exists with correct content
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Verify temp file removed
	assert.NoFileExists(t, tempName)
}

// TestAbort_Success verifies cleanup
func TestAbort_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)

	tempName := af.File.Name()

	require.NoError(t, af.Abort())

	// Verify temp file removed
	assert.NoFileExists(t, tempName)

	// Verify target not created
	assert.NoFileExists(t, path)
}

// TestAbort_ErrorHandling tests error capture
func TestAbort_ErrorHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)

	// Close file to force close error
	af.File.Close()

	// Remove temp file to force remove error
	os.Remove(af.File.Name())

	err = af.Abort()
	// Should get both errors
	require.Error(t, err)
	assert.Contains(t, err.Error(), "abort failed")
}

// TestClose_CloseError verifies cleanup on close failure
func TestClose_CloseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)

	tempName := af.File.Name()

	// Close file to force close error
	af.File.Close()

	err = af.Close()
	require.Error(t, err)

	// Verify temp file cleaned up even on error
	assert.NoFileExists(t, tempName)
}

// TestReadFrom verifies io.Copy integration
func TestReadFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)
	defer func() { _ = af.Abort() }()

	content := []byte("test content from reader")
	n, err := af.ReadFrom(bytes.NewReader(content))
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), n)
}

// TestFilePermissions verifies mode is set correctly
func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0600)
	require.NoError(t, err)

	_, err = af.Write([]byte("test"))
	require.NoError(t, err)

	require.NoError(t, af.Close())

	info, err := os.Stat(path)
	require.NoError(t, err)

	// On Unix, check exact permissions
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0600), mode)
	}
}

// TestMultipleAbortsSafe verifies calling Abort multiple times is safe
func TestMultipleAbortsSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	require.NoError(t, err)

	tempName := af.File.Name()

	// First abort should succeed
	require.NoError(t, af.Abort())
	assert.NoFileExists(t, tempName, "temp file should be removed after first abort")

	// Second abort should handle gracefully (file already gone)
	err = af.Abort()
	// Error is acceptable since file is already removed, but it should not panic
	t.Logf("Second Abort() returned: %v", err)
}

// TestNoTempFilesAfterOperations verifies no .tmp-* files remain after operations
func TestNoTempFilesAfterOperations(t *testing.T) {
	const testIterations = 5

	tests := []struct {
		name      string
		operation func(*File) error
	}{
		{"close", (*File).Close},
		{"abort", (*File).Abort},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Perform multiple operations
			for i := 0; i < testIterations; i++ {
				path := filepath.Join(dir, fmt.Sprintf("test%d.txt", i))

				af, err := New(path, 0644)
				require.NoError(t, err)

				_, err = af.Write([]byte("test data"))
				require.NoError(t, err)

				require.NoError(t, tt.operation(af))
			}

			// Check for any .tmp-* files
			tmpFiles, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
			require.NoError(t, err)
			assert.Empty(t, tmpFiles, "should be no temp files after %s", tt.name)
		})
	}
}
