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
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	icore "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin/loader" // registers built-in plugins
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/libp2p/go-libp2p/core/peer"
)

/// ------ Setting up the IPFS Repo

func setupPlugins(externalPluginsPath string) error {
	plugins, err := loader.NewPluginLoader(filepath.Join(externalPluginsPath, "plugins"))
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

func createTempRepo() (string, error) {
	repoPath, err := os.MkdirTemp("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	identity, err := config.CreateIdentity(io.Discard, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
	})
	if err != nil {
		return "", err
	}
	cfg, err := config.InitWithIdentity(identity)
	if err != nil {
		return "", err
	}

	// TCP on loopback with a random port. QUIC/UDP is disabled because it can
	// be throttled on some networks; TCP is more reliable for local testing.
	cfg.Addresses.Swarm = []string{
		"/ip4/127.0.0.1/tcp/0",
	}
	cfg.Swarm.Transports.Network.QUIC = config.False
	cfg.Swarm.Transports.Network.Relay = config.False
	cfg.Swarm.Transports.Network.WebTransport = config.False
	cfg.Swarm.Transports.Network.WebRTCDirect = config.False
	cfg.Swarm.Transports.Network.Websocket = config.False
	cfg.AutoTLS.Enabled = config.False

	// No DHT: we connect peers by address, so content routing is not needed.
	cfg.Routing.Type = config.NewOptionalString("none")

	// No automatic bootstrap: we connect only the peers we need.
	cfg.Bootstrap = []string{}

	// Optional: enable experimental features by modifying cfg before Init, e.g.:
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
	}

	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

/// ------ Spawning the node

// createNode opens the repo at repoPath and starts an IPFS node.
func createNode(ctx context.Context, repoPath string) (*core.IpfsNode, error) {
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		return nil, err
	}

	nodeOptions := &core.BuildCfg{
		Online: true,
		// No routing: peers are connected directly by address.
		// In production use libp2p.DHTClientOption or libp2p.DHTOption
		// so the node can find content and peers on the wider network.
		Routing: libp2p.NilRouterOption,
		Repo:    repo,
	}

	return core.NewNode(ctx, nodeOptions)
}

var loadPluginsOnce sync.Once

// spawnEphemeral creates a temporary repo, starts a node, and returns its API.
func spawnEphemeral(ctx context.Context) (icore.CoreAPI, *core.IpfsNode, error) {
	var onceErr error
	loadPluginsOnce.Do(func() {
		onceErr = setupPlugins("")
	})
	if onceErr != nil {
		return nil, nil, onceErr
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Spawn a local peer using a temporary path, for testing purposes.
	ipfsA, nodeA, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn peer node: %s", err))
	}

	fmt.Println("Spawning Kubo node on a temporary repo")
	ipfsB, _, err := spawnEphemeral(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}

	fmt.Println("IPFS node is running")

	// Connect nodeB to nodeA before adding content. This lets the connection
	// finish its setup during the Add below, so the fetch in Part IV is fast.
	peerAddrs, err := ipfsA.Swarm().LocalAddrs(ctx)
	if err != nil {
		panic(fmt.Errorf("could not get peer addresses: %s", err))
	}
	peerMa := peerAddrs[0].String() + "/p2p/" + nodeA.Identity.String()
	fmt.Println("Connecting to peer...")
	if err := connectToPeers(ctx, ipfsB, []string{peerMa}); err != nil {
		panic(fmt.Errorf("failed to connect to peer: %s", err))
	}
	fmt.Println("Connected to peer")

	peerCidFile, err := ipfsA.Unixfs().Add(ctx,
		files.NewBytesFile([]byte("hello from ipfs 101 in Kubo")))
	if err != nil {
		panic(fmt.Errorf("could not add File: %s", err))
	}

	fmt.Printf("Added file to peer with CID %s\n", peerCidFile.String())

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

	/// --- Part IV: Getting a file from another IPFS node

	fmt.Println("\n-- Fetching content from nodeA via bitswap --")

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
