package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCase represents a single migration test case
type TestCase struct {
	Name        string
	InputConfig map[string]any
	Assertions  []ConfigAssertion
}

// ConfigAssertion represents an assertion about the migrated config
type ConfigAssertion struct {
	Path     string
	Expected any
}

// RunMigrationTest runs a migration test with the given test case
func RunMigrationTest(t *testing.T, migration Migration, tc TestCase) {
	t.Helper()

	// Convert input to JSON
	inputJSON, err := json.MarshalIndent(tc.InputConfig, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal input config: %v", err)
	}

	// Run the migration's convert function
	var output bytes.Buffer
	if baseMig, ok := migration.(*BaseMigration); ok {
		err = baseMig.Convert(bytes.NewReader(inputJSON), &output)
		if err != nil {
			t.Fatalf("migration failed: %v", err)
		}
	} else {
		t.Skip("migration is not a BaseMigration")
	}

	// Parse output
	var result map[string]any
	err = json.Unmarshal(output.Bytes(), &result)
	if err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	// Run assertions
	for _, assertion := range tc.Assertions {
		AssertConfigField(t, result, assertion.Path, assertion.Expected)
	}
}

// AssertConfigField asserts that a field in the config has the expected value
func AssertConfigField(t *testing.T, config map[string]any, path string, expected any) {
	t.Helper()

	actual, exists := GetField(config, path)
	if expected == nil {
		if exists {
			t.Errorf("expected field %s to not exist, but it has value: %v", path, actual)
		}
		return
	}

	if !exists {
		t.Errorf("expected field %s to exist with value %v, but it doesn't exist", path, expected)
		return
	}

	// Handle different types of comparisons
	switch exp := expected.(type) {
	case []string:
		actualSlice, ok := actual.([]interface{})
		if !ok {
			t.Errorf("field %s: expected []string, got %T", path, actual)
			return
		}
		if len(exp) != len(actualSlice) {
			t.Errorf("field %s: expected slice of length %d, got %d", path, len(exp), len(actualSlice))
			return
		}
		for i, expVal := range exp {
			if actualSlice[i] != expVal {
				t.Errorf("field %s[%d]: expected %v, got %v", path, i, expVal, actualSlice[i])
			}
		}
	case map[string]string:
		actualMap, ok := actual.(map[string]any)
		if !ok {
			t.Errorf("field %s: expected map, got %T", path, actual)
			return
		}
		for k, v := range exp {
			if actualMap[k] != v {
				t.Errorf("field %s[%s]: expected %v, got %v", path, k, v, actualMap[k])
			}
		}
	default:
		if actual != expected {
			t.Errorf("field %s: expected %v, got %v", path, expected, actual)
		}
	}
}

// GenerateTestConfig creates a basic test config with the given fields
func GenerateTestConfig(fields map[string]any) map[string]any {
	// Start with a minimal valid config
	config := map[string]any{
		"Identity": map[string]any{
			"PeerID": "QmTest",
		},
	}

	// Merge in the provided fields
	maps.Copy(config, fields)

	return config
}

// CreateTestRepo creates a temporary test repository with the given version and config
func CreateTestRepo(t *testing.T, version int, config map[string]any) string {
	t.Helper()

	tempDir := t.TempDir()

	// Write version file
	versionPath := filepath.Join(tempDir, "version")
	err := os.WriteFile(versionPath, []byte(fmt.Sprintf("%d", version)), 0644)
	if err != nil {
		t.Fatalf("failed to write version file: %v", err)
	}

	// Write config file
	configPath := filepath.Join(tempDir, "config")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	err = os.WriteFile(configPath, configData, 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return tempDir
}

// AssertMigrationSuccess runs a full migration and checks that it succeeds
func AssertMigrationSuccess(t *testing.T, migration Migration, fromVersion, toVersion int, inputConfig map[string]any) map[string]any {
	t.Helper()

	// Create test repo
	repoPath := CreateTestRepo(t, fromVersion, inputConfig)

	// Run migration
	opts := Options{
		Path:    repoPath,
		Verbose: false,
	}

	err := migration.Apply(opts)
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	// Check version was updated
	versionBytes, err := os.ReadFile(filepath.Join(repoPath, "version"))
	if err != nil {
		t.Fatalf("failed to read version file: %v", err)
	}
	actualVersion := string(versionBytes)
	if actualVersion != fmt.Sprintf("%d", toVersion) {
		t.Errorf("expected version %d, got %s", toVersion, actualVersion)
	}

	// Read and return the migrated config
	configBytes, err := os.ReadFile(filepath.Join(repoPath, "config"))
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var result map[string]any
	err = json.Unmarshal(configBytes, &result)
	if err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	return result
}

// AssertMigrationReversible checks that a migration can be reverted
func AssertMigrationReversible(t *testing.T, migration Migration, fromVersion, toVersion int, inputConfig map[string]any) {
	t.Helper()

	// Create test repo at target version
	repoPath := CreateTestRepo(t, toVersion, inputConfig)

	// Create backup file (simulating a previous migration)
	backupPath := filepath.Join(repoPath, fmt.Sprintf("config.%d-to-%d.bak", fromVersion, toVersion))
	originalConfig, err := json.MarshalIndent(inputConfig, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal original config: %v", err)
	}

	if err := os.WriteFile(backupPath, originalConfig, 0644); err != nil {
		t.Fatalf("failed to write backup file: %v", err)
	}

	// Run revert
	if err := migration.Revert(Options{Path: repoPath}); err != nil {
		t.Fatalf("revert failed: %v", err)
	}

	// Verify version was reverted
	versionBytes, err := os.ReadFile(filepath.Join(repoPath, "version"))
	if err != nil {
		t.Fatalf("failed to read version file: %v", err)
	}

	if actualVersion := string(versionBytes); actualVersion != fmt.Sprintf("%d", fromVersion) {
		t.Errorf("expected version %d after revert, got %s", fromVersion, actualVersion)
	}

	// Verify config was reverted
	configBytes, err := os.ReadFile(filepath.Join(repoPath, "config"))
	if err != nil {
		t.Fatalf("failed to read reverted config file: %v", err)
	}

	var revertedConfig map[string]any
	if err := json.Unmarshal(configBytes, &revertedConfig); err != nil {
		t.Fatalf("failed to unmarshal reverted config: %v", err)
	}

	// Compare reverted config with original
	compareConfigs(t, inputConfig, revertedConfig, "")
}

// compareConfigs recursively compares two config maps and reports differences
func compareConfigs(t *testing.T, expected, actual map[string]any, path string) {
	t.Helper()

	// Build current path helper
	buildPath := func(key string) string {
		if path == "" {
			return key
		}
		return path + "." + key
	}

	// Check all expected fields exist and match
	for key, expectedValue := range expected {
		currentPath := buildPath(key)

		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("reverted config missing field %s", currentPath)
			continue
		}

		switch exp := expectedValue.(type) {
		case map[string]any:
			act, ok := actualValue.(map[string]any)
			if !ok {
				t.Errorf("field %s: expected map, got %T", currentPath, actualValue)
				continue
			}
			compareConfigs(t, exp, act, currentPath)
		default:
			if !reflect.DeepEqual(expectedValue, actualValue) {
				t.Errorf("field %s: expected %v, got %v after revert",
					currentPath, expectedValue, actualValue)
			}
		}
	}

	// Check for unexpected fields using maps.Keys (Go 1.23+)
	for key := range actual {
		if _, exists := expected[key]; !exists {
			t.Errorf("reverted config has unexpected field %s", buildPath(key))
		}
	}
}
