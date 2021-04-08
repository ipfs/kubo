package ipfsfetcher

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
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
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const (
	// Default maximum download size
	defaultFetchLimit = 1024 * 1024 * 512
)

type IpfsFetcher struct {
	distPath string
	limit    int64
	peers    []string

	openOnce  sync.Once
	openErr   error
	closeOnce sync.Once
	closeErr  error

	ipfs         iface.CoreAPI
	ipfsTmpDir   string
	ipfsStopFunc func()
}

// NewIpfsFetcher creates a new IpfsFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
func NewIpfsFetcher(distPath string, fetchLimit int64, peers []string) *IpfsFetcher {
	f := &IpfsFetcher{
		limit:    defaultFetchLimit,
		distPath: migrations.LatestIpfsDist,
		peers:    peers,
	}

	if distPath != "" {
		if !strings.HasPrefix(distPath, "/") {
			distPath = "/" + distPath
		}
		f.distPath = distPath
	}

	if fetchLimit != 0 {
		if fetchLimit < 0 {
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
		f.ipfsTmpDir, f.openErr = initTempNode(ctx)
		if f.openErr != nil {
			return
		}

		f.ipfs, f.ipfsStopFunc, f.openErr = startTempNode(f.ipfsTmpDir, f.peers)
	})

	fmt.Printf("Fetching with IPFS: %q\n", filePath)

	if f.openErr != nil {
		return nil, f.openErr
	}

	iPath, err := parsePath(path.Join(f.distPath, filePath))
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
		return migrations.NewLimitReadCloser(fileNode, f.limit), nil
	}
	return fileNode, nil
}

func (f *IpfsFetcher) Close() error {
	f.closeOnce.Do(func() {
		if f.ipfsStopFunc != nil {
			// Tell ipfs node to stop and wait for it to stop
			f.ipfsStopFunc()
		}

		if f.ipfsTmpDir != "" {
			// Remove the temp ipfs dir
			f.closeErr = os.RemoveAll(f.ipfsTmpDir)
		}
	})
	return f.closeErr
}

func initTempNode(ctx context.Context) (string, error) {
	err := setupPlugins()
	if err != nil {
		return "", err
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

func startTempNode(repoDir string, peers []string) (iface.CoreAPI, func(), error) {
	// Open the repo
	r, err := fsrepo.Open(repoDir)
	if err != nil {
		return nil, nil, err
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
		return nil, nil, err
	}

	ifaceCore, err := coreapi.NewCoreAPI(node)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	stopFunc := func() {
		// Tell ipfs to stop
		cancel()
		// Wait until ipfs is stopped
		<-node.Context().Done()
	}

	if len(peers) != 0 {
		// Asynchronously connect to any specified peers
		go func() {
			if err := connect(ctxIpfsLife, ifaceCore, peers); err != nil {
				fmt.Fprintf(os.Stderr, "failed to connect to peers: %s", err)
			}
		}()
	}

	return ifaceCore, stopFunc, nil
}

func parsePath(fetchPath string) (ipath.Path, error) {
	ipfsPath := ipath.New(fetchPath)
	if ipfsPath.IsValid() == nil {
		return ipfsPath, nil
	}

	u, err := url.Parse(fetchPath)
	if err != nil {
		return nil, fmt.Errorf("%q could not be parsed: %s", fetchPath, err)
	}

	switch proto := u.Scheme; proto {
	case "ipfs", "ipld", "ipns":
		ipfsPath = ipath.New(path.Join("/", proto, u.Host, u.Path))
	case "http", "https":
		ipfsPath = ipath.New(u.Path)
	default:
		return nil, fmt.Errorf("%q is not recognized as an IPFS path", fetchPath)
	}
	return ipfsPath, ipfsPath.IsValid()
}

func setupPlugins() error {
	defaultPath, err := migrations.IpfsDir("")
	if err != nil {
		return err
	}

	// Load plugins. This will skip the repo if not available.
	//
	// TODO: Is there a better way to check it plugins are loaded first?
	plugins, err := loader.NewPluginLoader(filepath.Join(defaultPath, "plugins"))
	if err != nil {
		return fmt.Errorf("error loading plugins: %s", err)
	}

	if err := plugins.Initialize(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		fmt.Fprintln(os.Stderr, "Did not initialize plugins:", err)
		//return fmt.Errorf("error initializing plugins: %s", err)
		return nil
	}

	if err := plugins.Inject(); err != nil {
		// Need to ignore errors here because plugins may already be loaded when
		// run from ipfs daemon.
		fmt.Fprintln(os.Stderr, "Did not inject plugins:", err)
		//return fmt.Errorf("error initializing plugins: %s", err)
		return nil
	}

	return nil
}

func connect(ctx context.Context, ipfs iface.CoreAPI, peers []string) error {
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

	connErrs := make(chan error)
	for _, pi := range pinfos {
		go func(pi *peer.AddrInfo) {
			if err := ipfs.Swarm().Connect(ctx, *pi); err != nil {
				connErrs <- fmt.Errorf("cound not connec to %q: %s", pi.ID, err)
			} else {
				connErrs <- nil
			}
		}(pi)
	}

	var fails []string
	for i := 0; i < len(pinfos); i++ {
		err := <-connErrs
		if err != nil {
			fails = append(fails, err.Error())
		}
	}
	if len(fails) != 0 {
		return fmt.Errorf(strings.Join(fails, ", "))
	}

	return nil
}
