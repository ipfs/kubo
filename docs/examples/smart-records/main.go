package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	config "github.com/ipfs/go-ipfs-config"
	ipfslibp2p "github.com/ipfs/go-ipfs/core/node/libp2p"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/libp2p/go-smart-record/ir"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/plugin/loader" // This package is needed so that all the preloaded plugins are loaded automatically
	"github.com/ipfs/go-ipfs/repo/fsrepo"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/metrics"
	"github.com/libp2p/go-libp2p-core/peer"
)

// IPFSNode structure.
type IPFSNode struct {
	Node  *core.IpfsNode
	API   icore.CoreAPI
	Close func() error
}

// CreateTempRepo creates a new repo in /tmp/
func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}

// CreateIPFSNode an IPFS specifying exchange node and returns its coreAPI
func CreateIPFSNode(ctx context.Context) (*IPFSNode, error) {

	repoPath, err := createTempRepo(ctx)
	if err != nil {
		return nil, err
	}
	repo, err := fsrepo.Open(repoPath)
	swarmAddrs := []string{
		"/ip4/0.0.0.0/tcp/0",
		"/ip6/::/tcp/0",
		"/ip4/0.0.0.0/udp/0/quic",
		"/ip6/::/udp/0/quic",
	}
	if err := repo.SetConfigKey("Addresses.Swarm", swarmAddrs); err != nil {
		return nil, err
	}
	if err := repo.SetConfigKey("Discovery.MDNS.Enabled", false); err != nil {
		return nil, err
	}

	// Construct the node
	nodeOptions := &core.BuildCfg{
		Online: true,
		// Routing: ipfslibp2p.DHTOption,
		Routing: ipfslibp2p.NilRouterOption,
		Repo:    repo,
	}

	node, err := core.NewNode(ctx, nodeOptions)
	fmt.Println("Listening at: ", node.PeerHost.Addrs())
	for _, i := range node.PeerHost.Addrs() {
		a := strings.Split(i.String(), "/")
		if a[1] == "ip4" && a[2] == "127.0.0.1" && a[3] == "tcp" {
			fmt.Println("Connect from other peers using: ")
			fmt.Printf("connect_/ip4/127.0.0.1/tcp/%v/p2p/%s\n", a[4], node.PeerHost.ID().Pretty())
		}

	}
	fmt.Println("PeerInfo: ", host.InfoFromHost(node.PeerHost))
	if err != nil {
		return nil, fmt.Errorf("Failed starting the node: %s", err)
	}

	api, err := coreapi.NewCoreAPI(node)
	// Attach the Core API to the constructed node
	return &IPFSNode{node, api, node.Close}, nil
}

// setupPlugins automatically loads plugins.
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

// PrintStats for the node.
func printStats(bs *metrics.Stats) {
	fmt.Printf("Bandwidth")
	fmt.Printf("TotalIn: %s\n", humanize.Bytes(uint64(bs.TotalIn)))
	fmt.Printf("TotalOut: %s\n", humanize.Bytes(uint64(bs.TotalOut)))
	fmt.Printf("RateIn: %s/s\n", humanize.Bytes(uint64(bs.RateIn)))
	fmt.Printf("RateOut: %s/s\n", humanize.Bytes(uint64(bs.RateOut)))
}

// conectPeer connects to a peer in the network.
func connectPeer(ctx context.Context, ipfs *IPFSNode, id string) error {
	maddr, err := ma.NewMultiaddr(id)
	if err != nil {
		fmt.Println("Invalid peer ID")
		return err
	}
	fmt.Println("Multiaddr", maddr)
	addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		fmt.Println("Invalid peer info", err)
		return err
	}
	err = ipfs.API.Swarm().Connect(ctx, *addrInfo)
	if err != nil {
		fmt.Println("Couldn't connect to peer", err)
		return err
	}
	fmt.Println("Connected successfully to peer")
	return nil
}

/// -------

func main() {
	if err := setupPlugins(""); err != nil {
		panic(fmt.Errorf("Failed setting up plugins: %s", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Spawn nodes using a temporary path, creating a temporary repo for the run
	fmt.Println("[*] Spawning IPFS nodes")
	node1, err := CreateIPFSNode(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}
	node2, err := CreateIPFSNode(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to spawn ephemeral node: %s", err))
	}
	defer node1.Close()
	defer node2.Close()

	time.Sleep(2 * time.Second)

	fmt.Println("[*] Connecting both nodes")
	err = node1.API.Swarm().Connect(ctx, *host.InfoFromHost(node2.Node.PeerHost))
	if err != nil {
		fmt.Println("Couldn't connect to peer", err)
		panic(err)
	}
	fmt.Println("Connected successfully to peer")

	vm := node1.Node.SmartRecordClient

	in1 := ir.Dict{
		Pairs: ir.Pairs{
			ir.Pair{Key: ir.String{Value: "peer1"}, Value: ir.String{Value: "Qmfoo"}},
		},
	}
	in2 := ir.Dict{
		Pairs: ir.Pairs{
			ir.Pair{Key: ir.String{Value: "peer2"}, Value: ir.String{Value: "QmBar"}},
		},
	}
	in := ir.Dict{
		Pairs: ir.Pairs{
			ir.Pair{Key: ir.String{Value: "peer1"}, Value: ir.String{Value: "Qmfoo"}},
			ir.Pair{Key: ir.String{Value: "peer2"}, Value: ir.String{Value: "QmBar"}},
		},
	}

	var w bytes.Buffer
	in1.WritePretty(&w)
	fmt.Println("[*] Updating in1 234 from node1 in node2")
	fmt.Println(w.String())
	w.Reset()
	err = vm.Update(ctx, "234", node2.Node.Identity, in1)
	if err != nil {
		panic(err)
	}
	in2.WritePretty(&w)
	fmt.Println("[*] Updating in2 234 from node1 in node2")
	fmt.Println(w.String())
	w.Reset()

	err = vm.Update(ctx, "234", node2.Node.Identity, in2)
	if err != nil {
		panic(err)
	}
	out, err := vm.Get(ctx, "234", node2.Node.Identity)
	if err != nil {
		panic(err)
	}
	fmt.Println("[*] Getting 234 from node2")

	for k, v := range *out {
		var w bytes.Buffer
		v.WritePretty(&w)
		fmt.Printf("(%s): %s", k.String(), string(w.Bytes()))
		w.Reset()
	}
	in.WritePretty(&w)
	fmt.Println(w.String())
	w.Reset()
	// TODO: Compare if the update was successful comparing with in once VM implementation
	// is done
	fmt.Println("Update went successfully")
	select {}
}
