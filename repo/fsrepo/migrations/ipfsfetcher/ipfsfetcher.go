package ipfsfetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/ipfs/go-libipfs/files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	ipath "github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/repo/fsrepo"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

const (
	// Default maximum download size
	defaultFetchLimit = 1024 * 1024 * 512

	tempNodeTCPAddr = "/ip4/127.0.0.1/tcp/0"
)

type IpfsFetcher struct {
	distPath       string
	limit          int64
	repoRoot       *string
	userConfigFile string

	openOnce  sync.Once
	openErr   error
	closeOnce sync.Once
	closeErr  error

	ipfs         iface.CoreAPI
	ipfsTmpDir   string
	ipfsStopFunc func()

	fetched []ipath.Path
	mutex   sync.Mutex

	addrInfo peer.AddrInfo
}

var _ migrations.Fetcher = (*IpfsFetcher)(nil)

// NewIpfsFetcher creates a new IpfsFetcher
//
// Specifying "" for distPath sets the default IPNS path.
// Specifying 0 for fetchLimit sets the default, -1 means no limit.
//
// Bootstrap and peer information in read from the IPFS config file in
// repoRoot, unless repoRoot is nil.  If repoRoot is empty (""), then read the
// config from the default IPFS directory.
func NewIpfsFetcher(distPath string, fetchLimit int64, repoRoot *string, userConfigFile string) *IpfsFetcher {
	f := &IpfsFetcher{
		limit:          defaultFetchLimit,
		distPath:       migrations.LatestIpfsDist,
		repoRoot:       repoRoot,
		userConfigFile: userConfigFile,
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
// site configured for this HttpFetcher.
func (f *IpfsFetcher) Fetch(ctx context.Context, filePath string) ([]byte, error) {
	// Initialize and start IPFS node on first call to Fetch, since the fetcher
	// may be created by not used.
	f.openOnce.Do(func() {
		bootstrap, peers := readIpfsConfig(f.repoRoot, f.userConfigFile)
		f.ipfsTmpDir, f.openErr = initTempNode(ctx, bootstrap, peers)
		if f.openErr != nil {
			return
		}

		f.openErr = f.startTempNode(ctx)
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

	f.recordFetched(iPath)

	fileNode, ok := nd.(files.File)
	if !ok {
		return nil, fmt.Errorf("%q is not a file", filePath)
	}

	var rc io.ReadCloser
	if f.limit != 0 {
		rc = migrations.NewLimitReadCloser(fileNode, f.limit)
	} else {
		rc = fileNode
	}
	defer rc.Close()

	return io.ReadAll(rc)
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

func (f *IpfsFetcher) AddrInfo() peer.AddrInfo {
	return f.addrInfo
}

// FetchedPaths returns the IPFS paths of all items fetched by this fetcher
func (f *IpfsFetcher) FetchedPaths() []ipath.Path {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.fetched
}

func (f *IpfsFetcher) recordFetched(fetchedPath ipath.Path) {
	// Mutex protects against update by concurrent calls to Fetch
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.fetched = append(f.fetched, fetchedPath)
}

func initTempNode(ctx context.Context, bootstrap []string, peers []peer.AddrInfo) (string, error) {
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

	// create temporary ipfs directory
	dir, err := os.MkdirTemp("", "ipfs-temp")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// configure the temporary node
	cfg.Routing.Type = config.NewOptionalString("dhtclient")

	// Disable listening for inbound connections
	cfg.Addresses.Gateway = []string{}
	cfg.Addresses.API = []string{}
	cfg.Addresses.Swarm = []string{tempNodeTCPAddr}

	if len(bootstrap) != 0 {
		cfg.Bootstrap = bootstrap
	}

	if len(peers) != 0 {
		cfg.Peering.Peers = peers
	}

	// Assumes that repo plugins are already loaded
	err = fsrepo.Init(dir, cfg)
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("failed to initialize ephemeral node: %s", err)
	}

	return dir, nil
}

func (f *IpfsFetcher) startTempNode(ctx context.Context) error {
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

	ipfs, err := coreapi.NewCoreAPI(node)
	if err != nil {
		cancel()
		return err
	}

	stopFunc := func() {
		// Tell ipfs to stop
		cancel()
		// Wait until ipfs is stopped
		<-node.Context().Done()
	}

	addrs, err := ipfs.Swarm().LocalAddrs(ctx)
	if err != nil {
		// Failure to get the local swarm address only means that the
		// downloaded migrations cannot be fetched through the temporary node.
		// So, print the error message and keep going.
		fmt.Fprintln(os.Stderr, "cannot get local swarm address:", err)
	}

	f.addrInfo = peer.AddrInfo{
		ID:    node.Identity,
		Addrs: addrs,
	}

	f.ipfs = ipfs
	f.ipfsStopFunc = stopFunc

	return nil
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
	default:
		return nil, fmt.Errorf("%q is not an IPFS path", fetchPath)
	}
	return ipfsPath, ipfsPath.IsValid()
}

func readIpfsConfig(repoRoot *string, userConfigFile string) (bootstrap []string, peers []peer.AddrInfo) {
	if repoRoot == nil {
		return
	}

	cfgPath, err := config.Filename(*repoRoot, userConfigFile)
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
		// If Seek fails, only log the error and continue on to try to read the
		// peering config anyway as it might still be readable
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
