package config

const DefaultMigrationKeep = "cache"

// DefaultMigrationDownloadSources defines the default download sources for legacy migrations (repo versions <16).
// Only HTTPS is supported for legacy migrations. IPFS downloads are not supported.
var DefaultMigrationDownloadSources = []string{"HTTPS"}

// Migration configures how legacy migrations are downloaded (repo versions <16).
//
// DEPRECATED: This configuration only applies to legacy external migrations for repository
// versions below 16. Modern repositories (v16+) use embedded migrations that do not require
// external downloads. These settings will be ignored for modern repository versions.
type Migration struct {
	// DEPRECATED: This field is deprecated and ignored for modern repositories (repo versions ≥16).
	DownloadSources []string `json:",omitempty"`
	// DEPRECATED: This field is deprecated and ignored for modern repositories (repo versions ≥16).
	Keep string `json:",omitempty"`
}
