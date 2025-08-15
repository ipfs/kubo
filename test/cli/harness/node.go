package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	serial "github.com/ipfs/kubo/config/serialize"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var log = logging.Logger("testharness")

// Node is a single Kubo node.
// Each node has its own config and can run its own Kubo daemon.
type Node struct {
	ID  int
	Dir string

	APIListenAddr     multiaddr.Multiaddr
	GatewayListenAddr multiaddr.Multiaddr
	SwarmAddr         multiaddr.Multiaddr
	EnableMDNS        bool

	IPFSBin string
	Runner  *Runner

	Daemon *RunResult
}

func BuildNode(ipfsBin, baseDir string, id int) *Node {
	dir := filepath.Join(baseDir, strconv.Itoa(id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}

	env := environToMap(os.Environ())
	env["IPFS_PATH"] = dir

	// If using "ipfs" binary name, provide helpful binary information
	if ipfsBin == "ipfs" {
		// Check if cmd/ipfs/ipfs exists (simple relative path check)
		localBinary := "cmd/ipfs/ipfs"
		localExists := false
		if _, err := os.Stat(localBinary); err == nil {
			localExists = true
			if abs, err := filepath.Abs(localBinary); err == nil {
				localBinary = abs
			}
		}

		// Check if ipfs is available in PATH
		pathBinary, pathErr := exec.LookPath("ipfs")

		// Handle different scenarios
		if pathErr != nil {
			// No ipfs in PATH
			if localExists {
				fmt.Printf("WARNING: No 'ipfs' found in PATH, but local binary exists at %s\n", localBinary)
				fmt.Printf("Consider adding it to PATH or run: export PATH=\"$(pwd)/cmd/ipfs:$PATH\"\n")
			} else {
				fmt.Printf("ERROR: No 'ipfs' binary found in PATH and no local build at cmd/ipfs/ipfs\n")
				fmt.Printf("Run 'make build' first or install ipfs and add it to PATH\n")
				panic("ipfs binary not available")
			}
		} else {
			// ipfs found in PATH
			if localExists && localBinary != pathBinary {
				fmt.Printf("NOTE: Local binary at %s differs from PATH binary at %s\n", localBinary, pathBinary)
				fmt.Printf("Consider adding the local binary to PATH if you want to use the version built by 'make build'\n")
			}
			// If they match or no local binary, no message needed
		}
	}

	return &Node{
		ID:      id,
		Dir:     dir,
		IPFSBin: ipfsBin,
		Runner: &Runner{
			Env: env,
			Dir: dir,
		},
	}
}

func (n *Node) WriteBytes(filename string, b []byte) {
	f, err := os.Create(filepath.Join(n.Dir, filename))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_, err = io.Copy(f, bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
}

// ReadFile reads the specific file. If it is relative, it is relative the node's root dir.
func (n *Node) ReadFile(filename string) string {
	f := filename
	if !filepath.IsAbs(filename) {
		f = filepath.Join(n.Dir, filename)
	}
	b, err := os.ReadFile(f)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (n *Node) ConfigFile() string {
	return filepath.Join(n.Dir, "config")
}

func (n *Node) ReadConfig() *config.Config {
	cfg, err := serial.Load(filepath.Join(n.Dir, "config"))
	if err != nil {
		panic(err)
	}
	return cfg
}

func (n *Node) WriteConfig(c *config.Config) {
	err := serial.WriteConfigFile(filepath.Join(n.Dir, "config"), c)
	if err != nil {
		panic(err)
	}
}

func (n *Node) UpdateConfig(f func(cfg *config.Config)) {
	cfg := n.ReadConfig()
	f(cfg)
	n.WriteConfig(cfg)
}

func (n *Node) ReadUserResourceOverrides() *rcmgr.PartialLimitConfig {
	var r rcmgr.PartialLimitConfig
	err := serial.ReadConfigFile(filepath.Join(n.Dir, "libp2p-resource-limit-overrides.json"), &r)
	switch err {
	case nil, serial.ErrNotInitialized:
		return &r
	default:
		panic(err)
	}
}

func (n *Node) WriteUserSuppliedResourceOverrides(c *rcmgr.PartialLimitConfig) {
	err := serial.WriteConfigFile(filepath.Join(n.Dir, "libp2p-resource-limit-overrides.json"), c)
	if err != nil {
		panic(err)
	}
}

func (n *Node) UpdateUserSuppliedResourceManagerOverrides(f func(overrides *rcmgr.PartialLimitConfig)) {
	overrides := n.ReadUserResourceOverrides()
	f(overrides)
	n.WriteUserSuppliedResourceOverrides(overrides)
}

func (n *Node) IPFS(args ...string) *RunResult {
	res := n.RunIPFS(args...)
	n.Runner.AssertNoError(res)
	return res
}

func (n *Node) PipeStrToIPFS(s string, args ...string) *RunResult {
	return n.PipeToIPFS(strings.NewReader(s), args...)
}

func (n *Node) PipeToIPFS(reader io.Reader, args ...string) *RunResult {
	res := n.RunPipeToIPFS(reader, args...)
	n.Runner.AssertNoError(res)
	return res
}

func (n *Node) RunPipeToIPFS(reader io.Reader, args ...string) *RunResult {
	return n.Runner.Run(RunRequest{
		Path:    n.IPFSBin,
		Args:    args,
		CmdOpts: []CmdOpt{RunWithStdin(reader)},
	})
}

func (n *Node) RunIPFS(args ...string) *RunResult {
	return n.Runner.Run(RunRequest{
		Path: n.IPFSBin,
		Args: args,
	})
}

// Init initializes and configures the IPFS node, after which it is ready to run.
func (n *Node) Init(ipfsArgs ...string) *Node {
	n.Runner.MustRun(RunRequest{
		Path: n.IPFSBin,
		Args: append([]string{"init"}, ipfsArgs...),
	})

	if n.SwarmAddr == nil {
		swarmAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if err != nil {
			panic(err)
		}
		n.SwarmAddr = swarmAddr
	}

	if n.APIListenAddr == nil {
		apiAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if err != nil {
			panic(err)
		}
		n.APIListenAddr = apiAddr
	}

	if n.GatewayListenAddr == nil {
		gatewayAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if err != nil {
			panic(err)
		}
		n.GatewayListenAddr = gatewayAddr
	}

	n.UpdateConfig(func(cfg *config.Config) {
		cfg.Bootstrap = []string{}
		cfg.Addresses.Swarm = []string{n.SwarmAddr.String()}
		cfg.Addresses.API = []string{n.APIListenAddr.String()}
		cfg.Addresses.Gateway = []string{n.GatewayListenAddr.String()}
		cfg.Swarm.DisableNatPortMap = true
		cfg.Discovery.MDNS.Enabled = n.EnableMDNS
		cfg.Routing.LoopbackAddressesOnLanDHT = config.True
	})
	return n
}

// StartDaemonWithReq runs a Kubo daemon with the given request.
// This overwrites the request Path with the Kubo bin path.
//
// For example, if you want to run the daemon and see stderr and stdout to debug:
//
//	 node.StartDaemonWithReq(harness.RunRequest{
//	 	 CmdOpts: []harness.CmdOpt{
//			harness.RunWithStderr(os.Stdout),
//			harness.RunWithStdout(os.Stdout),
//		 },
//	 })
func (n *Node) StartDaemonWithReq(req RunRequest, authorization string) *Node {
	alive := n.IsAlive()
	if alive {
		log.Panicf("node %d is already running", n.ID)
	}
	newReq := req
	newReq.Path = n.IPFSBin
	newReq.Args = append([]string{"daemon"}, req.Args...)
	newReq.RunFunc = (*exec.Cmd).Start

	log.Debugf("starting node %d", n.ID)
	res := n.Runner.MustRun(newReq)

	n.Daemon = res

	log.Debugf("node %d started, checking API", n.ID)
	n.WaitOnAPI(authorization)
	return n
}

func (n *Node) StartDaemon(ipfsArgs ...string) *Node {
	return n.StartDaemonWithReq(RunRequest{
		Args: ipfsArgs,
	}, "")
}

func (n *Node) StartDaemonWithAuthorization(secret string, ipfsArgs ...string) *Node {
	return n.StartDaemonWithReq(RunRequest{
		Args: ipfsArgs,
	}, secret)
}

func (n *Node) signalAndWait(watch <-chan struct{}, signal os.Signal, t time.Duration) bool {
	err := n.Daemon.Cmd.Process.Signal(signal)
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			log.Debugf("process for node %d has already finished", n.ID)
			return true
		}
		log.Panicf("error killing daemon for node %d with peer ID %s: %s", n.ID, n.PeerID(), err.Error())
	}
	timer := time.NewTimer(t)
	defer timer.Stop()
	select {
	case <-watch:
		return true
	case <-timer.C:
		return false
	}
}

func (n *Node) StopDaemon() *Node {
	log.Debugf("stopping node %d", n.ID)
	if n.Daemon == nil {
		log.Debugf("didn't stop node %d since no daemon present", n.ID)
		return n
	}
	watch := make(chan struct{}, 1)
	go func() {
		_, _ = n.Daemon.Cmd.Process.Wait()
		watch <- struct{}{}
	}()

	// os.Interrupt does not support interrupts on Windows https://github.com/golang/go/issues/46345
	if runtime.GOOS == "windows" {
		if n.signalAndWait(watch, syscall.SIGKILL, 5*time.Second) {
			return n
		}
		log.Panicf("timed out stopping node %d with peer ID %s", n.ID, n.PeerID())
	}

	log.Debugf("signaling node %d with SIGTERM", n.ID)
	if n.signalAndWait(watch, syscall.SIGTERM, 1*time.Second) {
		return n
	}
	log.Debugf("signaling node %d with SIGTERM", n.ID)
	if n.signalAndWait(watch, syscall.SIGTERM, 2*time.Second) {
		return n
	}
	log.Debugf("signaling node %d with SIGQUIT", n.ID)
	if n.signalAndWait(watch, syscall.SIGQUIT, 5*time.Second) {
		return n
	}
	log.Debugf("signaling node %d with SIGKILL", n.ID)
	if n.signalAndWait(watch, syscall.SIGKILL, 5*time.Second) {
		return n
	}
	log.Panicf("timed out stopping node %d with peer ID %s", n.ID, n.PeerID())
	return n
}

func (n *Node) APIAddr() multiaddr.Multiaddr {
	ma, err := n.TryAPIAddr()
	if err != nil {
		panic(err)
	}
	return ma
}

func (n *Node) APIURL() string {
	apiAddr := n.APIAddr()
	netAddr, err := manet.ToNetAddr(apiAddr)
	if err != nil {
		panic(err)
	}
	return "http://" + netAddr.String()
}

func (n *Node) TryAPIAddr() (multiaddr.Multiaddr, error) {
	b, err := os.ReadFile(filepath.Join(n.Dir, "api"))
	if err != nil {
		return nil, err
	}
	ma, err := multiaddr.NewMultiaddr(string(b))
	if err != nil {
		return nil, err
	}
	return ma, nil
}

func (n *Node) checkAPI(authorization string) bool {
	apiAddr, err := n.TryAPIAddr()
	if err != nil {
		log.Debugf("node %d API addr not available yet: %s", n.ID, err.Error())
		return false
	}

	if unixAddr, err := apiAddr.ValueForProtocol(multiaddr.P_UNIX); err == nil {
		parts := strings.SplitN(unixAddr, "/", 2)
		if len(parts) < 1 {
			panic("malformed unix socket address")
		}
		fileName := "/" + parts[1]
		_, err := os.Stat(fileName)
		return !errors.Is(err, fs.ErrNotExist)
	}

	ip, err := apiAddr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		panic(err)
	}
	port, err := apiAddr.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("http://%s:%s/api/v0/id", ip, port)
	log.Debugf("checking API for node %d at %s", n.ID, url)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		panic(err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Debugf("node %d API check error: %s", err.Error())
		return false
	}
	defer httpResp.Body.Close()
	resp := struct {
		ID string
	}{}

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		log.Debugf("error reading API check response for node %d: %s", n.ID, err.Error())
		return false
	}
	log.Debugf("got API check response for node %d: %s", n.ID, string(respBytes))

	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		log.Debugf("error decoding API check response for node %d: %s", n.ID, err.Error())
		return false
	}
	if resp.ID == "" {
		log.Debugf("API check response for node %d did not contain a Peer ID", n.ID)
		return false
	}
	respPeerID, err := peer.Decode(resp.ID)
	if err != nil {
		panic(err)
	}

	peerID := n.PeerID()
	if respPeerID != peerID {
		log.Panicf("expected peer ID %s but got %s", peerID, resp.ID)
	}

	log.Debugf("API check for node %d successful", n.ID)
	return true
}

func (n *Node) PeerID() peer.ID {
	cfg := n.ReadConfig()
	id, err := peer.Decode(cfg.Identity.PeerID)
	if err != nil {
		panic(err)
	}
	return id
}

func (n *Node) WaitOnAPI(authorization string) *Node {
	log.Debugf("waiting on API for node %d", n.ID)
	for i := 0; i < 50; i++ {
		if n.checkAPI(authorization) {
			log.Debugf("daemon API found, daemon stdout: %s", n.Daemon.Stdout.String())
			return n
		}
		time.Sleep(400 * time.Millisecond)
	}
	log.Panicf("node %d with peer ID %s failed to come online: \n%s\n\n%s", n.ID, n.PeerID(), n.Daemon.Stderr.String(), n.Daemon.Stdout.String())
	return n
}

func (n *Node) IsAlive() bool {
	if n.Daemon == nil || n.Daemon.Cmd == nil || n.Daemon.Cmd.Process == nil {
		return false
	}
	log.Debugf("signaling node %d daemon process for liveness check", n.ID)
	err := n.Daemon.Cmd.Process.Signal(syscall.Signal(0))
	if err == nil {
		log.Debugf("node %d daemon is alive", n.ID)
		return true
	}
	log.Debugf("node %d daemon not alive: %s", err.Error())
	return false
}

func (n *Node) SwarmAddrs() []multiaddr.Multiaddr {
	res := n.Runner.Run(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "addrs", "local"},
	})
	if res.ExitCode() != 0 {
		// If swarm command fails (e.g., daemon not online), return empty slice
		log.Debugf("Node %d: swarm addrs local failed (exit %d): %s", n.ID, res.ExitCode(), res.Stderr.String())
		return []multiaddr.Multiaddr{}
	}
	out := strings.TrimSpace(res.Stdout.String())
	if out == "" {
		log.Debugf("Node %d: swarm addrs local returned empty output", n.ID)
		return []multiaddr.Multiaddr{}
	}
	log.Debugf("Node %d: swarm addrs local output: %s", n.ID, out)
	outLines := strings.Split(out, "\n")
	var addrs []multiaddr.Multiaddr
	for _, addrStr := range outLines {
		addrStr = strings.TrimSpace(addrStr)
		if addrStr == "" {
			continue
		}
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			panic(err)
		}
		addrs = append(addrs, ma)
	}
	log.Debugf("Node %d: parsed %d swarm addresses", n.ID, len(addrs))
	return addrs
}

// SwarmAddrsWithTimeout waits for swarm addresses to be available
func (n *Node) SwarmAddrsWithTimeout(timeout time.Duration) []multiaddr.Multiaddr {
	start := time.Now()
	for time.Since(start) < timeout {
		addrs := n.SwarmAddrs()
		if len(addrs) > 0 {
			return addrs
		}
		time.Sleep(100 * time.Millisecond)
	}
	return []multiaddr.Multiaddr{}
}

func (n *Node) SwarmAddrsWithPeerIDs() []multiaddr.Multiaddr {
	return n.SwarmAddrsWithPeerIDsTimeout(5 * time.Second)
}

func (n *Node) SwarmAddrsWithPeerIDsTimeout(timeout time.Duration) []multiaddr.Multiaddr {
	ipfsProtocol := multiaddr.ProtocolWithCode(multiaddr.P_IPFS).Name
	peerID := n.PeerID()
	var addrs []multiaddr.Multiaddr
	for _, ma := range n.SwarmAddrsWithTimeout(timeout) {
		// add the peer ID to the multiaddr if it doesn't have it
		_, err := ma.ValueForProtocol(multiaddr.P_IPFS)
		if errors.Is(err, multiaddr.ErrProtocolNotFound) {
			comp, err := multiaddr.NewComponent(ipfsProtocol, peerID.String())
			if err != nil {
				panic(err)
			}
			ma = ma.Encapsulate(comp)
		}
		addrs = append(addrs, ma)
	}
	return addrs
}

func (n *Node) SwarmAddrsWithoutPeerIDs() []multiaddr.Multiaddr {
	var addrs []multiaddr.Multiaddr
	for _, ma := range n.SwarmAddrs() {
		i := 0
		for _, c := range ma {
			if c.Protocol().Code == multiaddr.P_IPFS {
				continue
			}
			ma[i] = c
			i++
		}
		ma = ma[:i]
		if len(ma) > 0 {
			addrs = append(addrs, ma)
		}
	}
	return addrs
}

func (n *Node) Connect(other *Node) *Node {
	// Get the peer addresses to connect to
	addrs := other.SwarmAddrsWithPeerIDs()
	if len(addrs) == 0 {
		// If no addresses available, skip connection
		log.Debugf("No swarm addresses available for connection")
		return n
	}
	// Use Run instead of MustRun to avoid panics on connection failures
	res := n.Runner.Run(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "connect", addrs[0].String()},
	})
	if res.ExitCode() != 0 {
		log.Debugf("swarm connect failed: %s", res.Stderr.String())
	}
	return n
}

// ConnectAndWait connects to another node and waits for the connection to be established
func (n *Node) ConnectAndWait(other *Node, timeout time.Duration) error {
	// Get the peer addresses to connect to - wait up to half the timeout for addresses
	addrs := other.SwarmAddrsWithPeerIDsTimeout(timeout / 2)
	if len(addrs) == 0 {
		return fmt.Errorf("no swarm addresses available for node %d after waiting %v", other.ID, timeout/2)
	}

	otherPeerID := other.PeerID()

	// Try to connect
	res := n.Runner.Run(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "connect", addrs[0].String()},
	})
	if res.ExitCode() != 0 {
		return fmt.Errorf("swarm connect failed: %s", res.Stderr.String())
	}

	// Wait for connection to be established
	start := time.Now()
	for time.Since(start) < timeout {
		peers := n.Peers()
		for _, peerAddr := range peers {
			if peerID, err := peerAddr.ValueForProtocol(multiaddr.P_P2P); err == nil {
				if peerID == otherPeerID.String() {
					return nil // Connection established
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for connection to node %d (peer %s)", other.ID, otherPeerID)
}

func (n *Node) Peers() []multiaddr.Multiaddr {
	// Wait for daemon to be ready if it's supposed to be running
	if n.Daemon != nil && n.Daemon.Cmd != nil && n.Daemon.Cmd.Process != nil {
		// Give daemon a short time to become ready
		for i := 0; i < 10; i++ {
			if n.IsAlive() {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	res := n.Runner.Run(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "peers"},
	})
	if res.ExitCode() != 0 {
		// If swarm peers fails (e.g., daemon not online), return empty slice
		log.Debugf("swarm peers failed: %s", res.Stderr.String())
		return []multiaddr.Multiaddr{}
	}
	var addrs []multiaddr.Multiaddr
	for _, line := range res.Stdout.Lines() {
		ma, err := multiaddr.NewMultiaddr(line)
		if err != nil {
			panic(err)
		}
		addrs = append(addrs, ma)
	}
	return addrs
}

func (n *Node) PeerWith(other *Node) {
	n.UpdateConfig(func(cfg *config.Config) {
		var addrs []multiaddr.Multiaddr
		for _, addrStr := range other.ReadConfig().Addresses.Swarm {
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				panic(err)
			}
			addrs = append(addrs, ma)
		}

		cfg.Peering.Peers = append(cfg.Peering.Peers, peer.AddrInfo{
			ID:    other.PeerID(),
			Addrs: addrs,
		})
	})
}

func (n *Node) Disconnect(other *Node) {
	n.IPFS("swarm", "disconnect", "/p2p/"+other.PeerID().String())
}

// GatewayURL waits for the gateway file and then returns its contents or times out.
func (n *Node) GatewayURL() string {
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			panic("timeout waiting for gateway file")
		default:
			b, err := os.ReadFile(filepath.Join(n.Dir, "gateway"))
			if err == nil {
				return strings.TrimSpace(string(b))
			}
			if !errors.Is(err, fs.ErrNotExist) {
				panic(err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func (n *Node) GatewayClient() *HTTPClient {
	return &HTTPClient{
		Client:  http.DefaultClient,
		BaseURL: n.GatewayURL(),
	}
}

func (n *Node) APIClient() *HTTPClient {
	return &HTTPClient{
		Client:  http.DefaultClient,
		BaseURL: n.APIURL(),
	}
}
