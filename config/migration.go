package config

import "github.com/libp2p/go-libp2p-core/peer"

const DefaultMigrationKeep = "cache"

var DefaultMigrationDownloadSources = []string{"HTTPS", "IPFS"}

// Migration configures how migrations are downloaded and if the downloads are
// added to IPFS locally
type Migration struct {
	// Sources in order of preference where "HTTPS" means our gateways and
	// "IPFS" means over IPFS. Any other values are interpretes as hostnames
	// for custom gateways. An empty list means "do the default thing"
	DownloadSources []string
	// Whether or not to keep the migration after downloading it.
	// Options are "discard", "cache", "pin".  Empty string for default.
	Keep string
	// Peers lists the nodes to attempt to connect with when downloading
	// migrations.
	Peers []peer.AddrInfo
}
