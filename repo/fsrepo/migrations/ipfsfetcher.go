package migrations

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ipfs/go-ipfs-config"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/node/libp2p"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsdir"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

type IpfsFetcher struct {
	distPath string
	limit    int64
	peers    []string

	openOnce  sync.Once
	closeOnce sync.Once
	err       error

	ipfs       iface.CoreAPI
	ipfsCancel context.CancelFunc
	ipfsCtx    context.Context
	ipfsTmpDir string
}

// NewIpfsFetcher creates a new IpfsFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
func NewIpfsFetcher(distPath string, fetchLimit int64, peers []string) *IpfsFetcher {
	f := &IpfsFetcher{
		limit:    defaultFetchLimit,
		distPath: LatestIpfsDist,
		peers:    peers,
	}

	if distPath != "" {
		if !strings.HasPrefix(distPath, "/") {
			distPath = "/" + distPath
		}
		f.distPath = distPath
	}

	if fetchLimit != 0 {
		if fetchLimit == -1 {
			fetchLimit = 0
		}
		f.limit = fetchLimit
	}

	return f
}

// Fetch attempts to fetch the file at the given path, from the distribution
// site configured for this HttpFetcher.  Returns io.ReadCloser on success,
// which caller must close.
func (f *IpfsFetcher) Fetch(ctx context.Context, filePath string) (io.ReadCloser, error) {
	// Initialize and start IPFS node on first call to Fetch, since the fetcher
	// may be created by not used.
	f.openOnce.Do(func() {
		f.ipfsTmpDir, f.err = initTempNode(ctx)
		if f.err != nil {
			return
		}

		f.err = f.startTempNode()
	})

	fmt.Printf("Fetching with IPFS: %q\n", filePath)

	if f.err != nil {
		return nil, f.err
	}

	iPath, err := parsePath(filepath.Join(f.distPath, filePath))
	if err != nil {
		return nil, err
	}

	nd, err := f.ipfs.Unixfs().Get(ctx, iPath)
	if err != nil {
		return nil, err
	}

	fileNode, ok := nd.(files.File)
	if !ok {
		return nil, fmt.Errorf("%q is not a file", filePath)
	}

	if f.limit != 0 {
		return NewLimitReadCloser(fileNode, f.limit), nil
	}
	return fileNode, nil
}

func (f *IpfsFetcher) Close() error {
	f.closeOnce.Do(func() {
		if f.ipfsCancel != nil {
			// Tell ipfs to stop
			f.ipfsCancel()

			// Wait until ipfs is stopped
			<-f.ipfsCtx.Done()
		}

		if f.ipfsTmpDir != "" {
			// Remove the temp ipfs dir
			if err := os.RemoveAll(f.ipfsTmpDir); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	})
	return nil
}

func initTempNode(ctx context.Context) (string, error) {
	defaultPath, err := ipfsdir.IpfsDir("")
	if err != nil {
		return "", err
	}

	// TODO: Is there a better way to check it plugins are loaded first?
	err = setupPlugins(defaultPath)
	// Need to ignore errors here because plugins may already be loaded when
	// run from ipfs daemon.
	if err != nil {
		fmt.Println("Ignored plugin error:", err)
	}

	identity, err := config.CreateIdentity(ioutil.Discard, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
	})
	if err != nil {
		return "", err
	}
	cfg, err := config.InitWithIdentity(identity)
	if err != nil {
		return "", err
	}

	// create temporary ipfs directory
	dir, err := ioutil.TempDir("", "ipfs-temp")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// configure the temporary node
	cfg.Routing.Type = "dhtclient"

	err = fsrepo.Init(dir, cfg)
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return dir, nil
}

func (f *IpfsFetcher) startTempNode() error {
	// Open the repo
	r, err := fsrepo.Open(f.ipfsTmpDir)
	if err != nil {
		return err
	}

	// Create a new lifetime context that is used to stop the temp ipfs node
	ctxIpfsLife, cancel := context.WithCancel(context.Background())

	// Construct the node
	node, err := core.NewNode(ctxIpfsLife, &core.BuildCfg{
		Online:  true,
		Routing: libp2p.DHTClientOption,
		Repo:    r,
	})
	if err != nil {
		cancel()
		r.Close()
		return err
	}

	ifaceCore, err := coreapi.NewCoreAPI(node)
	if err != nil {
		cancel()
		return err
	}

	f.ipfs = ifaceCore
	f.ipfsCancel = cancel      // stops node
	f.ipfsCtx = node.Context() // signals when node is stopped

	// Connect to any specified peers
	go func() {
		if err := connect(ctxIpfsLife, ifaceCore, f.peers); err != nil {
			fmt.Fprintf(os.Stderr, "failed to connect to peers: %s", err)
		}
	}()

	return nil
}

func parsePath(path string) (ipath.Path, error) {
	ipfsPath := ipath.New(path)
	if ipfsPath.IsValid() == nil {
		return ipfsPath, nil
	}

	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("%q could not be parsed: %s", path, err)
	}

	switch proto := u.Scheme; proto {
	case "ipfs", "ipld", "ipns":
		ipfsPath = ipath.New(filepath.Join("/", proto, u.Host, u.Path))
	case "http", "https":
		ipfsPath = ipath.New(u.Path)
	default:
		return nil, fmt.Errorf("%q is not recognized as an IPFS path", path)
	}
	return ipfsPath, ipfsPath.IsValid()
}

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

func connect(ctx context.Context, ipfs iface.CoreAPI, peers []string) error {
	if len(peers) == 0 {
		return nil
	}

	pinfos := make(map[peer.ID]*peer.AddrInfo, len(peers))
	for _, addrStr := range peers {
		addr, err := ma.NewMultiaddr(addrStr)
		if err != nil {
			return err
		}
		pii, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			return err
		}
		pi, ok := pinfos[pii.ID]
		if !ok {
			pi = &peer.AddrInfo{ID: pii.ID}
			pinfos[pi.ID] = pi
		}
		pi.Addrs = append(pi.Addrs, pii.Addrs...)
	}

	var wg sync.WaitGroup
	wg.Add(len(pinfos))
	for _, pi := range pinfos {
		go func(pi *peer.AddrInfo) {
			defer wg.Done()
			fmt.Fprintf(os.Stderr, "attempting to connect to peer: %q\n", pi)
			err := ipfs.Swarm().Connect(ctx, *pi)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect to %s: %s", pi.ID, err)
			}
			fmt.Fprintf(os.Stderr, "successfully connected to %s\n", pi.ID)
		}(pi)
	}
	wg.Wait()
	return nil
}
