package common

import (
	"fmt"
	"io"
	"path/filepath"
)

// BaseMigration provides common functionality for migrations
type BaseMigration struct {
	FromVersion string
	ToVersion   string
	Description string
	Convert     func(in io.ReadSeeker, out io.Writer) error
}

// Versions returns the version string for this migration
func (m *BaseMigration) Versions() string {
	return fmt.Sprintf("%s-to-%s", m.FromVersion, m.ToVersion)
}

// configBackupSuffix returns the backup suffix for the config file
// e.g. ".16-to-17.bak" results in "config.16-to-17.bak"
func (m *BaseMigration) configBackupSuffix() string {
	return fmt.Sprintf(".%s-to-%s.bak", m.FromVersion, m.ToVersion)
}

// Reversible returns true as we keep backups
func (m *BaseMigration) Reversible() bool {
	return true
}

// Apply performs the migration
func (m *BaseMigration) Apply(opts Options) error {
	if opts.Verbose {
		fmt.Printf("applying %s repo migration\n", m.Versions())
		if m.Description != "" {
			fmt.Printf("> %s\n", m.Description)
		}
	}

	// Check version
	if err := CheckVersion(opts.Path, m.FromVersion); err != nil {
		return err
	}

	configPath := filepath.Join(opts.Path, "config")

	// Perform migration with backup
	if err := WithBackup(configPath, m.configBackupSuffix(), m.Convert); err != nil {
		return err
	}

	// Update version
	if err := WriteVersion(opts.Path, m.ToVersion); err != nil {
		if opts.Verbose {
			fmt.Printf("failed to update version file to %s\n", m.ToVersion)
		}
		return err
	}

	if opts.Verbose {
		fmt.Println("updated version file")
		fmt.Printf("Migration %s succeeded\n", m.Versions())
	}

	return nil
}

// Revert reverts the migration
func (m *BaseMigration) Revert(opts Options) error {
	if opts.Verbose {
		fmt.Println("reverting migration")
	}

	// Check we're at the expected version
	if err := CheckVersion(opts.Path, m.ToVersion); err != nil {
		return err
	}

	// Restore backup
	configPath := filepath.Join(opts.Path, "config")
	if err := RevertBackup(configPath, m.configBackupSuffix()); err != nil {
		return err
	}

	// Revert version
	if err := WriteVersion(opts.Path, m.FromVersion); err != nil {
		return err
	}

	if opts.Verbose {
		fmt.Printf("lowered version number to %s\n", m.FromVersion)
	}

	return nil
}
