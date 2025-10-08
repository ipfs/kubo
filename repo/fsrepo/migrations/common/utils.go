package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/kubo/repo/fsrepo/migrations/atomicfile"
)

// CheckVersion verifies the repo is at the expected version
func CheckVersion(repoPath string, expectedVersion string) error {
	versionPath := filepath.Join(repoPath, "version")
	versionBytes, err := os.ReadFile(versionPath)
	if err != nil {
		return fmt.Errorf("could not read version file: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version != expectedVersion {
		return fmt.Errorf("expected version %s, got %s", expectedVersion, version)
	}
	return nil
}

// WriteVersion writes the version to the repo
func WriteVersion(repoPath string, version string) error {
	versionPath := filepath.Join(repoPath, "version")
	return os.WriteFile(versionPath, []byte(version), 0644)
}

// Must panics if the error is not nil. Use only for errors that cannot be handled gracefully.
func Must(err error) {
	if err != nil {
		panic(fmt.Errorf("error can't be dealt with transactionally: %w", err))
	}
}

// WithBackup performs a config file operation with automatic backup and rollback on error
func WithBackup(configPath string, backupSuffix string, fn func(in io.ReadSeeker, out io.Writer) error) error {
	// Read the entire file into memory first
	// This allows us to close the file before doing atomic operations,
	// which is necessary on Windows where open files can't be renamed
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Create an in-memory reader for the data
	in := bytes.NewReader(data)

	// Create backup atomically to prevent partial backup on interruption
	backupPath := configPath + backupSuffix
	backup, err := atomicfile.New(backupPath, 0600)
	if err != nil {
		return fmt.Errorf("failed to create backup file for %s: %w", backupPath, err)
	}
	if _, err := backup.Write(data); err != nil {
		Must(backup.Abort())
		return fmt.Errorf("failed to write backup data: %w", err)
	}
	if err := backup.Close(); err != nil {
		Must(backup.Abort())
		return fmt.Errorf("failed to finalize backup: %w", err)
	}

	// Create output file atomically
	out, err := atomicfile.New(configPath, 0600)
	if err != nil {
		// Clean up backup on error
		os.Remove(backupPath)
		return fmt.Errorf("failed to create atomic file for %s: %w", configPath, err)
	}

	// Run the conversion function
	if err := fn(in, out); err != nil {
		Must(out.Abort())
		// Clean up backup on error
		os.Remove(backupPath)
		return fmt.Errorf("config conversion failed: %w", err)
	}

	// Close the output file atomically
	Must(out.Close())
	// Backup remains for potential revert

	return nil
}

// RevertBackup restores a backup file
func RevertBackup(configPath string, backupSuffix string) error {
	return os.Rename(configPath+backupSuffix, configPath)
}

// ReadConfig reads and unmarshals a JSON config file into a map
func ReadConfig(r io.Reader) (map[string]any, error) {
	confMap := make(map[string]any)
	if err := json.NewDecoder(r).Decode(&confMap); err != nil {
		return nil, err
	}
	return confMap, nil
}

// WriteConfig marshals and writes a config map as indented JSON
func WriteConfig(w io.Writer, config map[string]any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(config)
}
