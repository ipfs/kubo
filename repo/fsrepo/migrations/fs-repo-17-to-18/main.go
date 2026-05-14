// Package main implements fs-repo-17-to-18 migration for IPFS repositories.
//
// This migration consolidates the Provider and Reprovider configurations into
// a unified Provide configuration section.
//
// Changes made:
//   - Migrates Provider.Enabled to Provide.Enabled
//   - Migrates Provider.WorkerCount to Provide.DHT.MaxWorkers
//   - Migrates Reprovider.Strategy to Provide.Strategy (converts "flat" to "all")
//   - Migrates Reprovider.Interval to Provide.DHT.Interval
//   - Removes deprecated Provider and Reprovider sections
//
// The migration is reversible and creates config.17-to-18.bak for rollback.
//
// Usage:
//
//	fs-repo-17-to-18 -path /path/to/ipfs/repo [-verbose] [-revert]
//
// This migration is embedded in Kubo and runs automatically during daemon startup.
// This standalone binary is provided for manual migration scenarios.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ipfs/kubo/repo/fsrepo/migrations/common"
	mg17 "github.com/ipfs/kubo/repo/fsrepo/migrations/fs-repo-17-to-18/migration"
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
		err = mg17.Migration.Revert(opts)
	} else {
		err = mg17.Migration.Apply(opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
		os.Exit(1)
	}
}
