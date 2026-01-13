package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasEmbeddedMigration(t *testing.T) {
	// Test that the 16-to-17 migration is registered
	assert.True(t, HasEmbeddedMigration("fs-repo-16-to-17"),
		"fs-repo-16-to-17 migration should be registered")

	// Test that a non-existent migration is not found
	assert.False(t, HasEmbeddedMigration("fs-repo-99-to-100"),
		"fs-repo-99-to-100 migration should not be registered")
}

func TestEmbeddedMigrations(t *testing.T) {
	// Test that we have at least one embedded migration
	assert.NotEmpty(t, embeddedMigrations, "No embedded migrations found")

	// Test that all registered migrations implement the interface
	for name, migration := range embeddedMigrations {
		assert.NotEmpty(t, migration.Versions(),
			"Migration %s has empty versions", name)
	}
}

func TestRunEmbeddedMigration(t *testing.T) {
	// Test that running a non-existent migration returns an error
	err := RunEmbeddedMigration(context.Background(), "non-existent", "/tmp", false)
	require.Error(t, err, "Expected error for non-existent migration")
}
