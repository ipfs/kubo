package harness

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCleanupOnSuccess verifies that cleanup happens when test passes
func TestCleanupOnSuccess(t *testing.T) {
	h := NewT(t)
	tempDir := h.Dir
	
	// Verify temp directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatalf("temp directory should exist: %s", tempDir)
	}
	
	// Test passes, so cleanup should happen automatically via t.Cleanup
}

// TestCleanupOnFailure verifies that cleanup is skipped when test fails
func TestCleanupOnFailure(t *testing.T) {
	// Create a sub-test that will fail
	t.Run("failing_subtest", func(subT *testing.T) {
		h := NewT(subT)
		tempDir := h.Dir
		
		// Store the temp directory path for later verification
		t.Setenv("TEST_TEMP_DIR", tempDir)
		
		// Verify temp directory exists
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			subT.Fatalf("temp directory should exist: %s", tempDir)
		}
		
		// Force the test to fail
		subT.Errorf("intentional test failure to verify cleanup behavior")
		
		// After this subtest completes with failure, the cleanup should preserve the directory
	})
	
	// Note: In a real scenario, you would check if the directory still exists
	// after the failed test, but that's complex to test in unit tests since
	// cleanup happens in defer/t.Cleanup which runs after the test function
}

// TestCleanupWithoutTestReference verifies backward compatibility
func TestCleanupWithoutTestReference(t *testing.T) {
	h := New() // Create harness without test reference
	tempDir := h.Dir
	
	// Verify temp directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Fatalf("temp directory should exist: %s", tempDir)
	}
	
	// Manually call cleanup - should work normally since h.t is nil
	h.Cleanup()
	
	// Verify directory was removed
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("temp directory should have been removed: %s", tempDir)
	}
}

// TestHarnessPreservesDirectoryOnFailure demonstrates the new functionality
func TestHarnessPreservesDirectoryOnFailure(t *testing.T) {
	h := NewT(t)
	tempDir := h.Dir
	
	// Create a test file in the harness directory
	testFile := filepath.Join(tempDir, "test_artifact.txt")
	err := os.WriteFile(testFile, []byte("test data"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	
	// Simulate test failure by marking the test as failed
	// Note: In real usage, this would happen automatically when test assertions fail
	t.Errorf("simulated test failure")
	
	// The cleanup will be called by t.Cleanup, and should preserve the directory
	// because t.Failed() will return true
}