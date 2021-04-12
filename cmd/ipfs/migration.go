package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
	"github.com/ipfs/interface-go-ipfs-core/options"
)

// getMigrationFetcher creates one or more fetchers according to
// downloadPolicy.
//
// The downloadPolicy parameter is a comma-separated string.  It may contain
// "ipfs" to indicate using the IpfsFetcher and "http" to indicate using the
// HttpFetcher.  Any other string is treated as a gateway URL to use with
// another HttpFetcher.  If downloadPolicy is is an empty string, then the
// default policy ("http,ipfs")is used.
//
// If the downloadPolicy requests an IpfsFetcher, and cfg is not nil, then an
// attempt is made to read the peers from the config.
func getMigrationFetcher(downloadPolicy string, cfg *config.Config) (migrations.Fetcher, error) {
	const httpUserAgent = "go-ipfs"

	var policyParts []string
	if downloadPolicy == "" {
		policyParts = []string{"http", "ipfs"}
	} else {
		policyParts = strings.Split(downloadPolicy, ",")
	}

	var fetchers []migrations.Fetcher
	seen := make(map[string]struct{})

	// Fetch migrations from current distribution, or location from environ
	fetchDistPath := migrations.GetDistPathEnv(migrations.CurrentIpfsDist)
	for _, policy := range policyParts {
		src := strings.TrimSpace(policy)
		if _, ok := seen[src]; ok {
			continue
		}
		seen[src] = struct{}{}

		switch src {
		case "ipfs":
			if cfg != nil {
				fetchers = append(fetchers, ipfsfetcher.NewIpfsFetcher(fetchDistPath, 0, cfg.Peering.Peers))
			} else {
				fetchers = append(fetchers, ipfsfetcher.NewIpfsFetcher(fetchDistPath, 0, nil))
			}
		case "http":
			fetchers = append(fetchers, migrations.NewHttpFetcher(fetchDistPath, "", httpUserAgent, 0))
		default:
			u, err := url.Parse(src)
			if err != nil {
				return nil, err
			}
			switch u.Scheme {
			case "":
				u.Scheme = "https"
			case "https", "http":
			default:
				return nil, errors.New("custom gateway scheme must be http or https")
			}
			fetchers = append(fetchers, migrations.NewHttpFetcher(fetchDistPath, u.String(), httpUserAgent, 0))
		}
	}

	if len(fetchers) == 1 {
		return fetchers[0], nil
	}

	// Wrap fetchers in a MultiFetcher to try them in order
	return migrations.NewMultiFetcher(fetchers...), nil
}

// addMigrationBins adds the files at binPaths to IPFS, pinning them also if
// pin is true.
func addMigrationBins(ctx context.Context, node *core.IpfsNode, binPaths []string, pin bool) error {
	ifaceCore, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}
	ufs := ifaceCore.Unixfs()

	// Add migration files
	for _, filePath := range binPaths {
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		ipfsPath, err := ufs.Add(ctx, files.NewReaderFile(f), options.Unixfs.Pin(pin))
		if err != nil {
			return err
		}
		fmt.Printf("Added migration file %q: %s\n", filepath.Base(filePath), ipfsPath)
	}

	return nil
}
