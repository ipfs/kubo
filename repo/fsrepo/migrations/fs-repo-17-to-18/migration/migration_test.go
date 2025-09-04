package mg17

import (
	"testing"

	"github.com/ipfs/kubo/repo/fsrepo/migrations/common"
)

func TestMigration17to18(t *testing.T) {
	migration := NewMigration()

	testCases := []common.TestCase{
		{
			Name: "Migrate Provider and Reprovider to Provide",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Provider": map[string]any{
					"Enabled":     true,
					"WorkerCount": 8,
					"Strategy":    "unused", // This field was unused and should be ignored
				},
				"Reprovider": map[string]any{
					"Strategy": "pinned",
					"Interval": "12h",
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide.Enabled", Expected: true},
				{Path: "Provide.DHT.MaxWorkers", Expected: float64(8)}, // JSON unmarshals to float64
				{Path: "Provide.Strategy", Expected: "pinned"},
				{Path: "Provide.DHT.Interval", Expected: "12h"},
				{Path: "Provider", Expected: nil},   // Should be deleted
				{Path: "Reprovider", Expected: nil}, // Should be deleted
			},
		},
		{
			Name: "Convert flat strategy to all",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Provider": map[string]any{
					"Enabled": false,
				},
				"Reprovider": map[string]any{
					"Strategy": "flat", // Deprecated, should be converted to "all"
					"Interval": "24h",
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide.Enabled", Expected: false},
				{Path: "Provide.Strategy", Expected: "all"}, // "flat" converted to "all"
				{Path: "Provide.DHT.Interval", Expected: "24h"},
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
			},
		},
		{
			Name: "Handle missing Provider section",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Reprovider": map[string]any{
					"Strategy": "roots",
					"Interval": "6h",
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide.Strategy", Expected: "roots"},
				{Path: "Provide.DHT.Interval", Expected: "6h"},
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
			},
		},
		{
			Name: "Handle missing Reprovider section",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Provider": map[string]any{
					"Enabled":     true,
					"WorkerCount": 16,
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide.Enabled", Expected: true},
				{Path: "Provide.DHT.MaxWorkers", Expected: float64(16)},
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
			},
		},
		{
			Name: "Handle empty Provider and Reprovider sections",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Provider":   map[string]any{},
				"Reprovider": map[string]any{},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide", Expected: nil}, // No fields to migrate
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
			},
		},
		{
			Name: "Handle missing both sections",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Datastore": map[string]any{
					"StorageMax": "10GB",
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide", Expected: nil}, // No Provider/Reprovider to migrate
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
				{Path: "Datastore.StorageMax", Expected: "10GB"}, // Other config preserved
			},
		},
		{
			Name: "Preserve other config sections",
			InputConfig: common.GenerateTestConfig(map[string]any{
				"Provider": map[string]any{
					"Enabled": true,
				},
				"Reprovider": map[string]any{
					"Strategy": "all",
				},
				"Swarm": map[string]any{
					"ConnMgr": map[string]any{
						"Type": "basic",
					},
				},
			}),
			Assertions: []common.ConfigAssertion{
				{Path: "Provide.Enabled", Expected: true},
				{Path: "Provide.Strategy", Expected: "all"},
				{Path: "Swarm.ConnMgr.Type", Expected: "basic"}, // Other config preserved
				{Path: "Provider", Expected: nil},
				{Path: "Reprovider", Expected: nil},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			common.RunMigrationTest(t, migration, tc)
		})
	}
}

func TestMigration17to18Reversible(t *testing.T) {
	migration := NewMigration()

	// Test that migration is reversible
	inputConfig := common.GenerateTestConfig(map[string]any{
		"Provide": map[string]any{
			"Enabled":     true,
			"WorkerCount": 8,
			"Strategy":    "pinned",
			"Interval":    "12h",
		},
	})

	// Test full migration and revert
	migratedConfig := common.AssertMigrationSuccess(t, migration, 17, 18, inputConfig)

	// Check that Provide section exists after migration
	common.AssertConfigField(t, migratedConfig, "Provide.Enabled", true)

	// Test revert
	common.AssertMigrationReversible(t, migration, 17, 18, migratedConfig)
}

func TestMigration17to18Integration(t *testing.T) {
	migration := NewMigration()

	// Test that the migration properly integrates with the common framework
	if migration.Versions() != "17-to-18" {
		t.Errorf("expected versions '17-to-18', got '%s'", migration.Versions())
	}

	if !migration.Reversible() {
		t.Error("migration should be reversible")
	}
}
