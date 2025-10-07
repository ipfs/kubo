package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNew_Success verifies atomic file creation
func TestNew_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = af.Abort() }()

	// Verify temp file exists
	if _, err := os.Stat(af.File.Name()); err != nil {
		t.Errorf("temp file not created: %v", err)
	}

	// Verify temp file is in same directory
	if filepath.Dir(af.File.Name()) != dir {
		t.Errorf("temp file in wrong dir: got %s, want %s",
			filepath.Dir(af.File.Name()), dir)
	}
}

// TestClose_Success verifies atomic replacement
func TestClose_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	content := []byte("test content")
	if _, err := af.Write(content); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	tempName := af.File.Name()

	if err := af.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify target file exists
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("target file not created: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("wrong content: got %q, want %q", data, content)
	}

	// Verify temp file removed
	if _, err := os.Stat(tempName); !os.IsNotExist(err) {
		t.Errorf("temp file not removed")
	}
}

// TestAbort_Success verifies cleanup
func TestAbort_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	tempName := af.File.Name()

	if err := af.Abort(); err != nil {
		t.Fatalf("Abort() failed: %v", err)
	}

	// Verify temp file removed
	if _, err := os.Stat(tempName); !os.IsNotExist(err) {
		t.Errorf("temp file not removed after abort")
	}

	// Verify target not created
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("target file should not exist after abort")
	}
}

// TestAbort_ErrorHandling tests error capture
func TestAbort_ErrorHandling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Close file to force close error
	af.File.Close()

	// Remove temp file to force remove error
	os.Remove(af.File.Name())

	err = af.Abort()
	// Should get both errors
	if err == nil {
		t.Error("Abort() should return error when both close and remove fail")
	}
	if !strings.Contains(err.Error(), "abort failed") {
		t.Errorf("expected 'abort failed' in error, got: %v", err)
	}
}

// TestClose_CloseError verifies cleanup on close failure
func TestClose_CloseError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	tempName := af.File.Name()

	// Close file to force close error
	af.File.Close()

	err = af.Close()
	if err == nil {
		t.Error("Close() should return error on close failure")
	}

	// Verify temp file cleaned up even on error
	if _, statErr := os.Stat(tempName); !os.IsNotExist(statErr) {
		t.Errorf("temp file should be removed on close error")
	}
}

// TestReadFrom verifies io.Copy integration
func TestReadFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0644)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer func() { _ = af.Abort() }()

	content := []byte("test content from reader")
	n, err := af.ReadFrom(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("ReadFrom() failed: %v", err)
	}
	if n != int64(len(content)) {
		t.Errorf("ReadFrom() wrote %d bytes, want %d", n, len(content))
	}
}

// TestFilePermissions verifies mode is set correctly
func TestFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	af, err := New(path, 0600)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if _, err := af.Write([]byte("test")); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}

	if err := af.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() failed: %v", err)
	}

	// On Unix, check exact permissions
	if runtime.GOOS != "windows" {
		mode := info.Mode().Perm()
		if mode != 0600 {
			t.Errorf("wrong permissions: got %o, want 0600", mode)
		}
	}
}
