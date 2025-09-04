// package mg17 contains the code to perform 17-18 repository migration in Kubo.
// This handles the following:
// - Migrate Provider and Reprovider configs to unified Provide config
// - Clear deprecated Provider and Reprovider fields
// - Increment repo version to 18
package mg17

import (
	"io"

	"github.com/ipfs/kubo/repo/fsrepo/migrations/common"
)

// Migration is the main exported migration for 17-to-18
var Migration = &common.BaseMigration{
	FromVersion: "17",
	ToVersion:   "18",
	Description: "Migrating Provider and Reprovider configuration to unified Provide configuration",
	Convert:     convert,
}

// NewMigration creates a new migration instance (for compatibility)
func NewMigration() common.Migration {
	return Migration
}

// convert performs the actual configuration transformation
func convert(in io.ReadSeeker, out io.Writer) error {
	// Read the configuration
	confMap, err := common.ReadConfig(in)
	if err != nil {
		return err
	}

	// Create new Provide section from Provider and Reprovider
	provide := make(map[string]any)

	// Migrate Provider fields if they exist
	provider := common.SafeCastMap(confMap["Provider"])
	if enabled, exists := provider["Enabled"]; exists {
		provide["Enabled"] = enabled
	}
	if workerCount, exists := provider["WorkerCount"]; exists {
		provide["WorkerCount"] = workerCount
	}
	// Note: Skip Provider.Strategy as it was unused

	// Migrate Reprovider fields if they exist
	reprovider := common.SafeCastMap(confMap["Reprovider"])
	if strategy, exists := reprovider["Strategy"]; exists {
		// Convert deprecated "flat" strategy to "all"
		if strategyStr, ok := strategy.(string); ok && strategyStr == "flat" {
			provide["Strategy"] = "all"
		} else {
			provide["Strategy"] = strategy
		}
	}
	if interval, exists := reprovider["Interval"]; exists {
		provide["Interval"] = interval
	}
	// Note: Sweep doesn't exist in master, it's new in this branch
	// So we don't need to migrate it from Reprovider

	// Set the new Provide section if we have any fields to migrate
	if len(provide) > 0 {
		confMap["Provide"] = provide
	}

	// Clear old Provider and Reprovider sections
	delete(confMap, "Provider")
	delete(confMap, "Reprovider")

	// Write the updated config
	return common.WriteConfig(out, confMap)
}
