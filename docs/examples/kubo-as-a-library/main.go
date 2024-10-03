package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	icore "github.com/ipfs/kubo/core/coreiface"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
)

/// ------ Setting up the IPFS Repo

func setupPlugins(externalPluginsPath string) error {
	// Load any external plugins if available on externalPluginsPath
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	// Load preloaded and external plugins
	if err := plugins.Initialize(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	if err := plugins.Inject(); err != nil {
		return fmt.Errorf("error initializing plugins: %s", err)
	}

	return nil
}

func createTempRepo() (string, error) {
	repoPath, err := os.MkdirTemp("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		return "", err
	}

	// When creating the repository, you can define custom settings on the repository, such as enabling experimental
	// features (See experimental-features.md) or customizing the gateway endpoint.
	// To do such things, you should modify the variable `cfg`. For example:
	if *flagExp {
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-filestore
		cfg.Experimental.FilestoreEnabled = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-urlstore
		cfg.Experimental.UrlstoreEnabled = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#ipfs-p2p
		cfg.Experimental.Libp2pStreamMounting = true
		// https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md#p2p-http-proxy
		cfg.Experimental.P2pHttpProxy = true
		// See also: https://github.com/ipfs/kubo/blob/master/docs/config.md
		// And: https://github.com/ipfs/kubo/blob/master/docs/experimental-features.md
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

/// ------ Spawning the node

// Creates an IPFS node and returns its coreAPI.
func createNode(ctx context.Context, repoPath string) (*core.IpfsNode, error) {
	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	// Construct the node

	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTOption, // This option sets the node to be a full DHT node (both fetching and storing DHT Records)
		// Routing: libp2p.DHTClientOption, // This option sets the node to be a client DHT node (only fetching records)
		Repo: repo,
	}

	return core.NewNode(ctx, nodeOptions)
}

var loadPluginsOnce sync.Once

// Spawns a node to be used just for this run (i.e. creates a tmp repo).
func spawnEphemeral(ctx context.Context) (icore.CoreAPI, *core.IpfsNode, error) {
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})
	if onceErr != nil {
		return nil, nil, onceErr
	}

	// Create a Temporary Repo
	repoPath, err := createTempRepo()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp repo: %s", err)
	}

	node, err := createNode(ctx, repoPath)
	if err != nil {
		return nil, nil, err
	}

	api, err := coreapi.NewCoreAPI(node)

	return api, node, err
}

func connectToPeers(ctx context.Context, ipfs icore.CoreAPI, peers []string) error {
	var wg sync.WaitGroup
	peerInfos := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := peerInfos[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			peerInfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	wg.Add(len(peerInfos))
	for _, peerInfo := range peerInfos {
		go func(peerInfo *peer.AddrInfo) {
			defer wg.Done()
			err := ipfs.Swarm().Connect(ctx, *peerInfo)
			if err != nil {
				log.Printf("failed to connect to %s: %s", peerInfo.ID, err)
			}
		}(peerInfo)
	}
	wg.Wait()
	return nil
}

func getUnixfsNode(path string) (files.Node, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	f, err := files.NewSerialFile(path, false, st)
	if err != nil {
		return nil, err
	}

	return f, nil
}

/// -------

var flagExp = flag.Bool("experimental", false, "enable experimental features")

func main() {
	flag.Parse()

	/// --- Part I: Getting a IPFS node running

	fmt.Println("-- Getting an IPFS node running -- ")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn a local peer using a temporary path, for testing purposes
	ipfsA, nodeA, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn peer node: %s", err))
	}

	peerCidFile, err := ipfsA.Unixfs().Add(ctx,
		files.NewBytesFile([]byte("hello from ipfs 101 in Kubo")))
	if err != nil {
		panic(fmt.Errorf("could not add File: %s", err))
	}

	fmt.Printf("Added file to peer with CID %s\n", peerCidFile.String())

	// Spawn a node using a temporary path, creating a temporary repo for the run
	fmt.Println("Spawning Kubo node on a temporary repo")
	ipfsB, _, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	fmt.Println("IPFS node is running")

	/// --- Part II: Adding a file and a directory to IPFS

	fmt.Println("\n-- Adding and getting back files & directories --")

	inputBasePath := "../example-folder/"
	inputPathFile := inputBasePath + "ipfs.paper.draft3.pdf"
	inputPathDirectory := inputBasePath + "test-dir"

	someFile, err := getUnixfsNode(inputPathFile)
	if err != nil {
		panic(fmt.Errorf("could not get File: %s", err))
	}

	cidFile, err := ipfsB.Unixfs().Add(ctx, someFile)
	if err != nil {
		panic(fmt.Errorf("could not add File: %s", err))
	}

	fmt.Printf("Added file to IPFS with CID %s\n", cidFile.String())

	someDirectory, err := getUnixfsNode(inputPathDirectory)
	if err != nil {
		panic(fmt.Errorf("could not get File: %s", err))
	}

	cidDirectory, err := ipfsB.Unixfs().Add(ctx, someDirectory)
	if err != nil {
		panic(fmt.Errorf("could not add Directory: %s", err))
	}

	fmt.Printf("Added directory to IPFS with CID %s\n", cidDirectory.String())

	/// --- Part III: Getting the file and directory you added back

	outputBasePath, err := os.MkdirTemp("", "example")
	if err != nil {
		panic(fmt.Errorf("could not create output dir (%v)", err))
	}
	fmt.Printf("output folder: %s\n", outputBasePath)
	outputPathFile := outputBasePath + strings.Split(cidFile.String(), "/")[2]
	outputPathDirectory := outputBasePath + strings.Split(cidDirectory.String(), "/")[2]

	rootNodeFile, err := ipfsB.Unixfs().Get(ctx, cidFile)
	if err != nil {
		panic(fmt.Errorf("could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNodeFile, outputPathFile)
	if err != nil {
		panic(fmt.Errorf("could not write out the fetched CID: %s", err))
	}

	fmt.Printf("got file back from IPFS (IPFS path: %s) and wrote it to %s\n", cidFile.String(), outputPathFile)

	rootNodeDirectory, err := ipfsB.Unixfs().Get(ctx, cidDirectory)
	if err != nil {
		panic(fmt.Errorf("could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNodeDirectory, outputPathDirectory)
	if err != nil {
		panic(fmt.Errorf("could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Got directory back from IPFS (IPFS path: %s) and wrote it to %s\n", cidDirectory.String(), outputPathDirectory)

	/// --- Part IV: Getting a file from the IPFS Network

	fmt.Println("\n-- Going to connect to a few nodes in the Network as bootstrappers --")

	peerMa := fmt.Sprintf("/ip4/127.0.0.1/udp/4010/p2p/%s", nodeA.Identity.String())

	bootstrapNodes := []string{
		// IPFS Bootstrapper nodes.
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		// "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",

		// IPFS Cluster Pinning nodes
		// "/ip4/138.201.67.219/tcp/4001/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		// "/ip4/138.201.67.219/udp/4001/quic/p2p/QmUd6zHcbkbcs7SMxwLs48qZVX3vpcM8errYS7xEczwRMA",
		// "/ip4/138.201.67.220/tcp/4001/p2p/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		// "/ip4/138.201.67.220/udp/4001/quic/p2p/QmNSYxZAiJHeLdkBg38roksAR9So7Y5eojks1yjEcUtZ7i",
		// "/ip4/138.201.68.74/tcp/4001/p2p/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		// "/ip4/138.201.68.74/udp/4001/quic/p2p/QmdnXwLrC8p1ueiq2Qya8joNvk3TVVDAut7PrikmZwubtR",
		// "/ip4/94.130.135.167/tcp/4001/p2p/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",
		// "/ip4/94.130.135.167/udp/4001/quic/p2p/QmUEMvxS2e7iDrereVYc5SWPauXPyNwxcy9BXZrC1QTcHE",

		// You can add more nodes here, for example, another IPFS node you might have running locally, mine was:
		// "/ip4/127.0.0.1/tcp/4010/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
		// "/ip4/127.0.0.1/udp/4010/quic/p2p/QmZp2fhDLxjYue2RiUvLwT9MWdnbDxam32qYFnGmxZDh5L",
		peerMa,
	}

	go func() {
		err := connectToPeers(ctx, ipfsB, bootstrapNodes)
		if err != nil {
			log.Printf("failed connect to peers: %s", err)
		}
	}()

	exampleCIDStr := peerCidFile.RootCid().String()

	fmt.Printf("Fetching a file from the network with CID %s\n", exampleCIDStr)
	outputPath := outputBasePath + exampleCIDStr
	testCID := path.FromCid(peerCidFile.RootCid())

	rootNode, err := ipfsB.Unixfs().Get(ctx, testCID)
	if err != nil {
		panic(fmt.Errorf("could not get file with CID: %s", err))
	}

	err = files.WriteTo(rootNode, outputPath)
	if err != nil {
		panic(fmt.Errorf("could not write out the fetched CID: %s", err))
	}

	fmt.Printf("Wrote the file to %s\n", outputPath)

	fmt.Println("\nAll done! You just finalized your first tutorial on how to use Kubo as a library")
}
