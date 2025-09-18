package common

import (
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
	in, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer in.Close()

	// Create backup
	backup, err := atomicfile.New(configPath+backupSuffix, 0600)
	if err != nil {
		return err
	}

	// Copy to backup
	if _, err := backup.ReadFrom(in); err != nil {
		Must(backup.Abort())
		return err
	}

	// Reset input for reading
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		Must(backup.Abort())
		return err
	}

	// Create output file
	out, err := atomicfile.New(configPath, 0600)
	if err != nil {
		Must(backup.Abort())
		return err
	}

	// Run the conversion function
	if err := fn(in, out); err != nil {
		Must(out.Abort())
		Must(backup.Abort())
		return err
	}

	// Close everything on success
	Must(out.Close())
	Must(backup.Close())

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
