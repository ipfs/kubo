// Package main implements fs-repo-16-to-17 migration for IPFS repositories.
//
// This migration transitions repositories from version 16 to 17, introducing
// the AutoConf system that replaces hardcoded network defaults with dynamic
// configuration fetched from autoconf.json.
//
// Changes made:
//   - Enables AutoConf system with default settings
//   - Migrates default bootstrap peers to "auto" sentinel value
//   - Sets DNS.Resolvers["."] to "auto" for dynamic DNS resolver configuration
//   - Migrates Routing.DelegatedRouters to ["auto"]
//   - Migrates Ipns.DelegatedPublishers to ["auto"]
//   - Preserves user customizations (custom bootstrap peers, DNS resolvers)
//
// The migration is reversible and creates config.16-to-17.bak for rollback.
//
// Usage:
//
//	fs-repo-16-to-17 -path /path/to/ipfs/repo [-verbose] [-revert]
//
// This migration is embedded in Kubo starting from version 0.37 and runs
// automatically during daemon startup. This standalone binary is provided
// for manual migration scenarios.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ipfs/kubo/repo/fsrepo/migrations/common"
	mg16 "github.com/ipfs/kubo/repo/fsrepo/migrations/fs-repo-16-to-17/migration"
)

func main() {
	var path = flag.String("path", "", "Path to IPFS repository")
	var verbose = flag.Bool("verbose", false, "Enable verbose output")
	var revert = flag.Bool("revert", false, "Revert migration")
	flag.Parse()

	if *path == "" {
		fmt.Fprintf(os.Stderr, "Error: -path flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	opts := common.Options{
		Path:    *path,
		Verbose: *verbose,
	}

	var err error
	if *revert {
		err = mg16.Migration.Revert(opts)
	} else {
		err = mg16.Migration.Apply(opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}
}
