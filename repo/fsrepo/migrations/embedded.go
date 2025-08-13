package migrations

import (
	"context"
	"fmt"
	"log"
	"os"

	mg16 "github.com/ipfs/kubo/repo/fsrepo/migrations/fs-repo-16-to-17/migration"
)

// EmbeddedMigration represents an embedded migration that can be run directly
type EmbeddedMigration interface {
	Versions() string
	Apply(opts mg16.Options) error
	Revert(opts mg16.Options) error
	Reversible() bool
}

// embeddedMigrations contains all embedded migrations
var embeddedMigrations = map[string]EmbeddedMigration{
	"fs-repo-16-to-17": &mg16.Migration{},
}

// RunEmbeddedMigration runs an embedded migration if available
func RunEmbeddedMigration(ctx context.Context, migrationName string, ipfsDir string, revert bool) error {
	migration, exists := embeddedMigrations[migrationName]
	if !exists {
		return fmt.Errorf("embedded migration %s not found", migrationName)
	}

	if revert && !migration.Reversible() {
		return fmt.Errorf("migration %s is not reversible", migrationName)
	}

	logger := log.New(os.Stdout, "", 0)
	logger.Printf("Running embedded migration %s...", migrationName)

	opts := mg16.Options{
		Path:    ipfsDir,
		Verbose: true,
	}

	var err error
	if revert {
		err = migration.Revert(opts)
	} else {
		err = migration.Apply(opts)
	}

	if err != nil {
		return fmt.Errorf("embedded migration %s failed: %w", migrationName, err)
	}

	logger.Printf("Embedded migration %s completed successfully", migrationName)
	return nil
}

// HasEmbeddedMigration checks if a migration is available as embedded
func HasEmbeddedMigration(migrationName string) bool {
	_, exists := embeddedMigrations[migrationName]
	return exists
}

// RunEmbeddedMigrations runs all needed embedded migrations from current version to target version.
//
// This function migrates an IPFS repository using embedded migrations that are built into the Kubo binary.
// Embedded migrations are available for repo version 17+ and provide fast, network-free migration execution.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - targetVer: Target repository version to migrate to
//   - ipfsDir: Path to the IPFS repository directory
//   - allowDowngrade: Whether to allow downgrade migrations (reduces target version)
//
// Returns:
//   - nil on successful migration
//   - error if migration fails, repo path is invalid, or no embedded migrations are available
//
// Behavior:
//   - Validates that ipfsDir contains a valid IPFS repository
//   - Determines current repository version automatically
//   - Returns immediately if already at target version
//   - Prevents downgrades unless allowDowngrade is true
//   - Runs all necessary migrations in sequence (e.g., 16→17→18 if going from 16 to 18)
//   - Creates backups and uses atomic operations to prevent corruption
//
// Error conditions:
//   - Repository path is invalid or inaccessible
//   - Current version cannot be determined
//   - Downgrade attempted with allowDowngrade=false
//   - No embedded migrations available for the version range
//   - Individual migration fails during execution
//
// Example:
//
//	err := RunEmbeddedMigrations(ctx, 17, "/path/to/.ipfs", false)
//	if err != nil {
//	    // Handle migration failure, may need to fall back to external migrations
//	}
func RunEmbeddedMigrations(ctx context.Context, targetVer int, ipfsDir string, allowDowngrade bool) error {
	ipfsDir, err := CheckIpfsDir(ipfsDir)
	if err != nil {
		return err
	}

	fromVer, err := RepoVersion(ipfsDir)
	if err != nil {
		return fmt.Errorf("could not get repo version: %w", err)
	}

	if fromVer == targetVer {
		return nil
	}

	revert := fromVer > targetVer
	if revert && !allowDowngrade {
		return fmt.Errorf("downgrade not allowed from %d to %d", fromVer, targetVer)
	}

	logger := log.New(os.Stdout, "", 0)
	logger.Print("Looking for embedded migrations.")

	migrations, _, err := findMigrations(ctx, fromVer, targetVer)
	if err != nil {
		return err
	}

	embeddedCount := 0
	for _, migrationName := range migrations {
		if HasEmbeddedMigration(migrationName) {
			err = RunEmbeddedMigration(ctx, migrationName, ipfsDir, revert)
			if err != nil {
				return err
			}
			embeddedCount++
		}
	}

	if embeddedCount == 0 {
		return fmt.Errorf("no embedded migrations found for version %d to %d", fromVer, targetVer)
	}

	logger.Printf("Success: fs-repo migrated to version %d using embedded migrations.\n", targetVer)
	return nil
}
