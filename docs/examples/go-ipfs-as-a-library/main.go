package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	config "github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	iCore "github.com/ipfs/interface-go-ipfs-core"
	iCorePath "github.com/ipfs/interface-go-ipfs-core/path"
)

/// ------ Spawning the node

func setupPlugins(path string) error {
	// Load plugins. This will skip the repo if not available.
	plugins, err := loader.NewPluginLoader(filepath.Join(path, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

type CfgOpt func(*config.Config)

func createNode(ctx context.Context, repoPath string) (iCore.CoreAPI, error) {
	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	// Construct the node
	node, err := core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		// Routing: libp2p.DHTClientOption,
		Repo: repo,
	})
	if err != nil {
		return nil, err
	}

	return coreapi.NewCoreAPI(node)
}

func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Set default config with option for 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// configure the temporary node
	// cfg.Routing.Type = "dhtclient"
	// cfg.Experimental.QUIC = true
	// cfg.Datastore.Spec = map[string]interface{}{
	// 	"type": "badgerds",
	// 	"path": "badger",
	// }

	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

/*
func spawnDefault(ctx context.Context) (iface.CoreAPI, error) {

}
*/

/*
func spawnEphemeral(ctx context.Context) (iface.CoreAPI, error) {
	defaultPath, err := config.PathRoot()
	if err != nil {
		// shouldn't be possible
		return nil, err
	}

	return tmpNode(ctx)
}
*/

func spawnDefaultOrEphemeral(ctx context.Context) (iCore.CoreAPI, error) {
	// Attempt to spawn a node in default location, check if repo already exists
	defaultPath, err := config.PathRoot()
	if err != nil {
		// shouldn't be possible
		return nil, err
	}

	ipfs, err := createNode(ctx, defaultPath)

	if err == nil {
		return ipfs, nil
	}

	// Spawn a node with a tmpRepo
	repoPath, err := createTempRepo(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to create temp repo", err)
	}

	// Spawning an ephemeral IPFS node
	return createNode(ctx, repoPath)
}

// ----- Writing to disk
func writeTo(nd files.Node, fpath string) error {
	switch nd := nd.(type) {
	case *files.Symlink:
		return os.Symlink(nd.Target, fpath)
	case files.File:
		f, err := os.Create(fpath)
		defer f.Close()
		if err != nil {
			return err
		}

		var r io.Reader = nd
		_, err = io.Copy(f, r)
		if err != nil {
			return err
		}
		return nil
	case files.Directory:
		err := os.Mkdir(fpath, 0777)
		if err != nil {
			return err
		}

		entries := nd.Entries()
		for entries.Next() {
			child := filepath.Join(fpath, entries.Name())
			if err := writeTo(entries.Node(), child); err != nil {
				return err
			}
		}
		return entries.Err()
	default:
		return fmt.Errorf("file type %T at %q is not supported", nd, fpath)
	}
}

/// -------

func main() {
	fmt.Println("Starting")

	if err := setupPlugins(""); err != nil {
		panic(err)
	}

	ctx, _ := context.WithCancel(context.Background())

	ipfs, err := spawnDefaultOrEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn node: %s", err))
	}

	fmt.Println("IPFS node running")

	testCIDStr := "QmUaoioqU7bxezBQZkUcgcSyokatMY71sxsALxQmRRrHrj"
	outputPath := "/Users/imp/Downloads/test-101/" + testCIDStr
	testCID := iCorePath.New(testCIDStr)

	out, err := ipfs.Unixfs().Get(ctx, testCID)
	if err != nil {
		panic(fmt.Errorf("Could not get CID: %s", err))
	}

	err = writeTo(out, outputPath)
	if err != nil {
		panic(fmt.Errorf("Could not write out the fetched CID: %s", err))
	}
}
