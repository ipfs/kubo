package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/peer"
)

// readMigrationConfig reads the migration config out of the config, avoiding
// reading anything other than the migration section. That way, we're free to
// make arbitrary changes to all _other_ sections in migrations.
func readMigrationConfig(repoRoot string) (*config.Migration, error) {
	var cfg struct {
		Migration config.Migration
	}

	cfgPath, err := config.Filename(repoRoot)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	err = json.NewDecoder(cfgFile).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.Migration.Keep {
	case "":
		cfg.Migration.Keep = config.DefaultMigrationKeep
	case "discard", "cache", "keep":
	default:
		return nil, errors.New("unknown config value, Migrations.Keep must be 'cache', 'pin', or 'discard'")
	}

	if len(cfg.Migration.DownloadSources) == 0 {
		cfg.Migration.DownloadSources = config.DefaultMigrationDownloadSources
	}

	return &cfg.Migration, nil
}

func readIpfsConfig(repoRoot *string) (bootstrap []string, peers []peer.AddrInfo) {
	if repoRoot == nil {
		return
	}

	cfgPath, err := config.Filename(*repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer cfgFile.Close()

	// Attempt to read bootstrap addresses
	var bootstrapCfg struct {
		Bootstrap []string
	}
	err = json.NewDecoder(cfgFile).Decode(&bootstrapCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read bootstrap peers from config")
	} else {
		bootstrap = bootstrapCfg.Bootstrap
	}

	if _, err = cfgFile.Seek(0, 0); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	// Attempt to read peers
	var peeringCfg struct {
		Peering config.Peering
	}
	err = json.NewDecoder(cfgFile).Decode(&peeringCfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot read peering from config")
	} else {
		peers = peeringCfg.Peering.Peers
	}

	return
}

// getMigrationFetcher creates one or more fetchers according to
// config.Migration.DownloadSources.  If an IpfsFetcher is required, then
// bootstrap and peer information in read from the config file in repoRoot,
// unless repoRoot is nil.
func getMigrationFetcher(cfg *config.Migration, repoRoot *string) (migrations.Fetcher, error) {
	const httpUserAgent = "go-ipfs"

	// Fetch migrations from current distribution, or location from environ
	fetchDistPath := migrations.GetDistPathEnv(migrations.CurrentIpfsDist)

	var fetchers []migrations.Fetcher
	for _, src := range cfg.DownloadSources {
		src := strings.TrimSpace(src)
		switch src {
		case "IPFS", "ipfs":
			bootstrap, peers := readIpfsConfig(repoRoot)
			fetchers = append(fetchers, ipfsfetcher.NewIpfsFetcher(fetchDistPath, 0, bootstrap, peers))
		case "HTTPS", "https", "HTTP", "http":
			fetchers = append(fetchers, migrations.NewHttpFetcher(fetchDistPath, "", httpUserAgent, 0))
		default:
			u, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("bad gateway address: %s", err)
			}
			switch u.Scheme {
			case "":
				u.Scheme = "https"
			case "https", "http":
			default:
				return nil, errors.New("bad gateway address: url scheme must be http or https")
			}
			fetchers = append(fetchers, migrations.NewHttpFetcher(fetchDistPath, u.String(), httpUserAgent, 0))
		case "":
			// Ignore empty string
		}
	}
	if len(fetchers) == 0 {
		return nil, errors.New("no sources specified")
	}

	if len(fetchers) == 1 {
		return fetchers[0], nil
	}

	// Wrap fetchers in a MultiFetcher to try them in order
	return migrations.NewMultiFetcher(fetchers...), nil
}

func addMigrations(ctx context.Context, node *core.IpfsNode, fetcher migrations.Fetcher, pin bool) error {
	var fetchers []migrations.Fetcher
	if mf, ok := fetcher.(*migrations.MultiFetcher); ok {
		fetchers = mf.Fetchers()
	} else {
		fetchers = []migrations.Fetcher{fetcher}
	}

	for _, fetcher := range fetchers {
		switch f := fetcher.(type) {
		case *ipfsfetcher.IpfsFetcher:
			// Add migrations by connecting to temp node and getting from IPFS
			err := addMigrationPaths(ctx, node, f.AddrInfo(), f.FetchedPaths(), pin)
			if err != nil {
				return err
			}
		case *migrations.HttpFetcher:
			// Add the downloaded migration files directly
			if migrations.DownloadDirectory != "" {
				var paths []string
				err := filepath.Walk(migrations.DownloadDirectory, func(filePath string, info os.FileInfo, err error) error {
					if info.IsDir() {
						return nil
					}
					paths = append(paths, filePath)
					return nil
				})
				if err != nil {
					return err
				}
				err = addMigrationFiles(ctx, node, paths, pin)
				if err != nil {
					return err
				}
			}
		default:
			return errors.New("Cannot get migrations from unknown fetcher type")
		}
	}

	return nil
}

// addMigrationFiles adds the files at paths to IPFS, optionally pinning them
func addMigrationFiles(ctx context.Context, node *core.IpfsNode, paths []string, pin bool) error {
	if len(paths) == 0 {
		return nil
	}
	ifaceCore, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}
	ufs := ifaceCore.Unixfs()

	// Add migration files
	for _, filePath := range paths {
		f, err := os.Open(filePath)
		if err != nil {
			return err
		}

		fi, err := f.Stat()
		if err != nil {
			return err
		}

		ipfsPath, err := ufs.Add(ctx, files.NewReaderStatFile(f, fi), options.Unixfs.Pin(pin))
		if err != nil {
			return err
		}
		fmt.Printf("Added migration file %q: %s\n", filepath.Base(filePath), ipfsPath)
	}

	return nil
}

// addMigrationPaths adds the files at paths to IPFS, optionally pinning
// them. This is done after connecting to the peer.
func addMigrationPaths(ctx context.Context, node *core.IpfsNode, peerInfo peer.AddrInfo, paths []ipath.Path, pin bool) error {
	if len(paths) == 0 {
		return errors.New("nothing downloaded by ipfs fetcher")
	}
	if len(peerInfo.Addrs) == 0 {
		return errors.New("no local swarm address for migration node")
	}

	ipfs, err := coreapi.NewCoreAPI(node)
	if err != nil {
		return err
	}

	// Connect to temp node
	if err := ipfs.Swarm().Connect(ctx, peerInfo); err != nil {
		return fmt.Errorf("could not connect to migration peer %q: %s", peerInfo.ID, err)
	}
	fmt.Printf("connected to migration peer %q\n", peerInfo)

	if pin {
		pinApi := ipfs.Pin()
		for _, ipfsPath := range paths {
			err := pinApi.Add(ctx, ipfsPath)
			if err != nil {
				return err
			}
			fmt.Printf("Added and pinned migration file: %q\n", ipfsPath)
		}
		return nil
	}

	ufs := ipfs.Unixfs()

	// Add migration files
	for _, ipfsPath := range paths {
		err = ipfsGet(ctx, ufs, ipfsPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func ipfsGet(ctx context.Context, ufs coreiface.UnixfsAPI, ipfsPath ipath.Path) error {
	nd, err := ufs.Get(ctx, ipfsPath)
	if err != nil {
		return err
	}
	defer nd.Close()

	fnd, ok := nd.(files.File)
	if !ok {
		return fmt.Errorf("not a file node: %q", ipfsPath)
	}
	_, err = io.Copy(ioutil.Discard, fnd)
	if err != nil {
		return fmt.Errorf("cannot read migration: %w", err)
	}
	fmt.Printf("Added migration file: %q\n", ipfsPath)
	return nil
}
