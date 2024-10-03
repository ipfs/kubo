package config

const DefaultMigrationKeep = "cache"

var DefaultMigrationDownloadSources = []string{"HTTPS", "IPFS"}

// Migration configures how migrations are downloaded and if the downloads are
// added to IPFS locally.
type Migration struct {
	// Sources in order of preference, where "IPFS" means use IPFS and "HTTPS"
	// means use default gateways. Any other values are interpreted as
	// hostnames for custom gateways. Empty list means "use default sources".
	DownloadSources []string
	// Whether or not to keep the migration after downloading it.
	// Options are "discard", "cache", "pin".  Empty string for default.
	Keep string
}
