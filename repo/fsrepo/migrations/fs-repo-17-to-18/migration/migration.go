// package mg17 contains the code to perform 17-18 repository migration in Kubo.
// This handles the following:
// - Migrate Provider and Reprovider configs to unified Provide config
// - Clear deprecated Provider and Reprovider fields
// - Increment repo version to 18
package mg17

import (
	"fmt"
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

	// Create new Provide section with DHT subsection from Provider and Reprovider
	provide := make(map[string]any)
	dht := make(map[string]any)
	hasNonDefaultValues := false

	// Migrate Provider fields if they exist
	provider := common.SafeCastMap(confMap["Provider"])
	if enabled, exists := provider["Enabled"]; exists {
		provide["Enabled"] = enabled
		// Log migration for non-default values
		if enabledBool, ok := enabled.(bool); ok && !enabledBool {
			fmt.Printf("  Migrated Provider.Enabled=%v to Provide.Enabled=%v\n", enabledBool, enabledBool)
			hasNonDefaultValues = true
		}
	}
	if workerCount, exists := provider["WorkerCount"]; exists {
		dht["MaxWorkers"] = workerCount
		// Log migration for all worker count values
		if count, ok := workerCount.(float64); ok {
			fmt.Printf("  Migrated Provider.WorkerCount=%v to Provide.DHT.MaxWorkers=%v\n", int(count), int(count))
			hasNonDefaultValues = true

			// Additional guidance for high WorkerCount
			if count > 5 {
				fmt.Printf("  ⚠️  For better resource utilization, consider enabling Provide.DHT.SweepEnabled=true\n")
				fmt.Printf("     and adjusting Provide.DHT.DedicatedBurstWorkers if announcement of new CIDs\n")
				fmt.Printf("     should take priority over periodic reprovide interval.\n")
			}
		}
	}
	// Note: Skip Provider.Strategy as it was unused

	// Migrate Reprovider fields if they exist
	reprovider := common.SafeCastMap(confMap["Reprovider"])
	if strategy, exists := reprovider["Strategy"]; exists {
		if strategyStr, ok := strategy.(string); ok {
			// Convert deprecated "flat" strategy to "all"
			if strategyStr == "flat" {
				provide["Strategy"] = "all"
				fmt.Printf("  Migrated deprecated Reprovider.Strategy=\"flat\" to Provide.Strategy=\"all\"\n")
			} else {
				// Migrate any other strategy value as-is
				provide["Strategy"] = strategyStr
				fmt.Printf("  Migrated Reprovider.Strategy=\"%s\" to Provide.Strategy=\"%s\"\n", strategyStr, strategyStr)
			}
			hasNonDefaultValues = true
		} else {
			// Not a string, set to default "all" to ensure valid config
			provide["Strategy"] = "all"
			fmt.Printf("  Warning: Reprovider.Strategy was not a string, setting Provide.Strategy=\"all\"\n")
			hasNonDefaultValues = true
		}
	}
	if interval, exists := reprovider["Interval"]; exists {
		dht["Interval"] = interval
		// Log migration for non-default intervals
		if intervalStr, ok := interval.(string); ok && intervalStr != "22h" && intervalStr != "" {
			fmt.Printf("  Migrated Reprovider.Interval=\"%s\" to Provide.DHT.Interval=\"%s\"\n", intervalStr, intervalStr)
			hasNonDefaultValues = true
		}
	}
	// Note: Sweep is a new field introduced in v0.38, not present in v0.37
	// So we don't need to migrate it from Reprovider

	// Set the DHT section if we have any DHT fields to migrate
	if len(dht) > 0 {
		provide["DHT"] = dht
	}

	// Set the new Provide section if we have any fields to migrate
	if len(provide) > 0 {
		confMap["Provide"] = provide
	}

	// Clear old Provider and Reprovider sections
	delete(confMap, "Provider")
	delete(confMap, "Reprovider")

	// Print documentation link if we migrated any non-default values
	if hasNonDefaultValues {
		fmt.Printf("  See: https://github.com/ipfs/kubo/blob/master/docs/config.md#provide\n")
	}

	// Write the updated config
	return common.WriteConfig(out, confMap)
}
