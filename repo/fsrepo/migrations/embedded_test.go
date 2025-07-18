package migrations

import (
	"context"
	"testing"
)

func TestHasEmbeddedMigration(t *testing.T) {
	// Test that the 16-to-17 migration is registered
	if !HasEmbeddedMigration("fs-repo-16-to-17") {
		t.Error("fs-repo-16-to-17 migration should be registered")
	}

	// Test that a non-existent migration is not found
	if HasEmbeddedMigration("fs-repo-99-to-100") {
		t.Error("fs-repo-99-to-100 migration should not be registered")
	}
}

func TestEmbeddedMigrations(t *testing.T) {
	// Test that we have at least one embedded migration
	if len(embeddedMigrations) == 0 {
		t.Error("No embedded migrations found")
	}

	// Test that all registered migrations implement the interface
	for name, migration := range embeddedMigrations {
		if migration.Versions() == "" {
			t.Errorf("Migration %s has empty versions", name)
		}
	}
}

func TestRunEmbeddedMigration(t *testing.T) {
	// Test that running a non-existent migration returns an error
	err := RunEmbeddedMigration(context.Background(), "non-existent", "/tmp", false)
	if err == nil {
		t.Error("Expected error for non-existent migration")
	}
}
