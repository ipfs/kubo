package harness

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ipfs/kubo/config"
	serial "github.com/ipfs/kubo/config/serialize"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

//var log = logging.Logger("testharness")

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
	Log     *TestLogger

	Daemon *RunResult
}

func buildNode(log *TestLogger, ipfsBin, baseDir string, id int) *Node {
	dir := filepath.Join(baseDir, strconv.Itoa(id))
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}

	env := environToMap(os.Environ())
	env["IPFS_PATH"] = dir

	log = log.AddPrefix(fmt.Sprintf("node %d", id))

	return &Node{
		ID:      id,
		Dir:     dir,
		IPFSBin: ipfsBin,
		Log:     log,
		Runner: &Runner{
			Log: log,
			Env: env,
			Dir: dir,
		},
	}
}

func (n *Node) ReadConfig() *config.Config {
	cfg, err := serial.Load(filepath.Join(n.Dir, "config"))
	if err != nil {
		n.Log.Fatal(err)
	}
	return cfg
}

func (n *Node) WriteConfig(c *config.Config) {
	err := serial.WriteConfigFile(filepath.Join(n.Dir, "config"), c)
	if err != nil {
		n.Log.Fatal(err)
	}
}

func (n *Node) UpdateConfig(f func(cfg *config.Config)) {
	cfg := n.ReadConfig()
	f(cfg)
	n.WriteConfig(cfg)
}

func (n *Node) IPFS(args ...string) RunResult {
	res := n.RunIPFS(args...)
	n.Runner.AssertNoError(res)
	return res
}

func (n *Node) PipeStrToIPFS(s string, args ...string) RunResult {
	return n.PipeToIPFS(strings.NewReader(s), args...)
}

func (n *Node) PipeToIPFS(reader io.Reader, args ...string) RunResult {
	res := n.RunPipeToIPFS(reader, args...)
	n.Runner.AssertNoError(res)
	return res
}

func (n *Node) RunPipeToIPFS(reader io.Reader, args ...string) RunResult {
	return n.Runner.Run(RunRequest{
		Path:    n.IPFSBin,
		Args:    args,
		CmdOpts: []CmdOpt{RunWithStdin(reader)},
	})
}

func (n *Node) RunIPFS(args ...string) RunResult {
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
			n.Log.Fatal(err)
		}
		n.SwarmAddr = swarmAddr
	}

	if n.APIListenAddr == nil {
		apiAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if err != nil {
			n.Log.Fatal(err)
		}
		n.APIListenAddr = apiAddr
	}

	if n.GatewayListenAddr == nil {
		gatewayAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
		if err != nil {
			n.Log.Fatal(err)
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
	})
	return n
}

func (n *Node) StartDaemon(ipfsArgs ...string) *Node {
	alive := n.IsAlive()
	if alive {
		n.Log.Fatal("daemon already running", n.ID)
	}

	daemonArgs := append([]string{"daemon"}, ipfsArgs...)
	n.Log.Log("starting daemon")
	res := n.Runner.MustRun(RunRequest{
		Path:    n.IPFSBin,
		Args:    daemonArgs,
		RunFunc: (*exec.Cmd).Start,
	})

	n.Daemon = &res

	n.Log.Log("started, checking API")
	n.WaitOnAPI()
	return n
}

func (n *Node) signalAndWait(watch <-chan struct{}, signal os.Signal, t time.Duration) bool {
	err := n.Daemon.Cmd.Process.Signal(signal)
	if err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			n.Log.Log("process has already finished")
			return true
		}
		n.Log.Fatalf("error killing daemon with peerID=%s: %s", n.PeerID(), err.Error())
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
	n.Log.Log("stopping daemon")
	if n.Daemon == nil {
		n.Log.Log("didn't stop node since no daemon present")
		return n
	}
	watch := make(chan struct{}, 1)
	go func() {
		_, _ = n.Daemon.Cmd.Process.Wait()
		watch <- struct{}{}
	}()
	n.Log.Log("signaling with SIGTERM")
	if n.signalAndWait(watch, syscall.SIGTERM, 1*time.Second) {
		return n
	}
	n.Log.Log("signaling with SIGTERM")
	if n.signalAndWait(watch, syscall.SIGTERM, 2*time.Second) {
		return n
	}
	n.Log.Log("signaling with SIGQUIT")
	if n.signalAndWait(watch, syscall.SIGQUIT, 5*time.Second) {
		return n
	}
	n.Log.Log("signaling with SIGKILL")
	if n.signalAndWait(watch, syscall.SIGKILL, 5*time.Second) {
		return n
	}
	n.Log.Fatalf("timed out stopping peerID=%s", n.PeerID())
	return n
}

func (n *Node) APIAddr() multiaddr.Multiaddr {
	ma, err := n.TryAPIAddr()
	if err != nil {
		n.Log.Fatal(err)
	}
	return ma
}

func (n *Node) APIURL() string {
	apiAddr := n.APIAddr()
	netAddr, err := manet.ToNetAddr(apiAddr)
	if err != nil {
		n.Log.Fatal(err)
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

func (n *Node) checkAPI() bool {
	apiAddr, err := n.TryAPIAddr()
	if err != nil {
		n.Log.Logf("API addr not available yet: %s", err.Error())
		return false
	}
	ip, err := apiAddr.ValueForProtocol(multiaddr.P_IP4)
	if err != nil {
		n.Log.Fatal(err)
	}
	port, err := apiAddr.ValueForProtocol(multiaddr.P_TCP)
	if err != nil {
		n.Log.Fatal(err)
	}
	url := fmt.Sprintf("http://%s:%s/api/v0/id", ip, port)
	n.Log.Logf("checking API at %s", url)
	httpResp, err := http.Post(url, "", nil)
	if err != nil {
		n.Log.Logf("API check error: %s", err.Error())
		return false
	}
	defer httpResp.Body.Close()
	resp := struct {
		ID string
	}{}

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		n.Log.Logf("error reading API check response: %s", err.Error())
		return false
	}
	n.Log.Logf("got API check response: %s", string(respBytes))

	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		n.Log.Logf("error decoding API check response: %s", err.Error())
		return false
	}
	if resp.ID == "" {
		n.Log.Logf("API check response for did not contain a Peer ID")
		return false
	}
	respPeerID, err := peer.Decode(resp.ID)
	if err != nil {
		n.Log.Fatal(err)
	}

	peerID := n.PeerID()
	if respPeerID != peerID {
		n.Log.Fatalf("expected peer ID %s but got %s", peerID, resp.ID)
	}

	n.Log.Log("API check successful")
	return true
}

func (n *Node) PeerID() peer.ID {
	cfg := n.ReadConfig()
	id, err := peer.Decode(cfg.Identity.PeerID)
	if err != nil {
		n.Log.Fatal(err)
	}
	return id
}

func (n *Node) WaitOnAPI() *Node {
	n.Log.Log("waiting on API")
	for i := 0; i < 50; i++ {
		if n.checkAPI() {
			n.Log.Logf("daemon API found, daemon stdout: %s", n.Daemon.Stdout.String())
			return n
		}
		time.Sleep(400 * time.Millisecond)
	}
	n.Log.Fatalf("node with peer ID %s failed to come online: \n%s\n\n%s", n.PeerID(), n.Daemon.Stderr.String(), n.Daemon.Stdout.String())
	return n
}

func (n *Node) IsAlive() bool {
	if n.Daemon == nil || n.Daemon.Cmd == nil || n.Daemon.Cmd.Process == nil {
		return false
	}
	n.Log.Log("signaling daemon process for liveness check")
	err := n.Daemon.Cmd.Process.Signal(syscall.Signal(0))
	if err == nil {
		n.Log.Log("daemon is alive")
		return true
	}
	n.Log.Logf("daemon not alive: %s", err.Error())
	return false
}

func (n *Node) SwarmAddrs() []multiaddr.Multiaddr {
	res := n.Runner.MustRun(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "addrs", "local"},
	})
	ipfsProtocol := multiaddr.ProtocolWithCode(multiaddr.P_IPFS).Name
	peerID := n.PeerID()
	out := strings.TrimSpace(res.Stdout.String())
	outLines := strings.Split(out, "\n")
	var addrs []multiaddr.Multiaddr
	for _, addrStr := range outLines {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			n.Log.Fatal(err)
		}

		// add the peer ID to the multiaddr if it doesn't have it
		_, err = ma.ValueForProtocol(multiaddr.P_IPFS)
		if errors.Is(err, multiaddr.ErrProtocolNotFound) {
			comp, err := multiaddr.NewComponent(ipfsProtocol, peerID.String())
			if err != nil {
				n.Log.Fatal(err)
			}
			ma = ma.Encapsulate(comp)
		}
		addrs = append(addrs, ma)
	}
	return addrs
}

func (n *Node) Connect(other *Node) *Node {
	n.Runner.MustRun(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "connect", other.SwarmAddrs()[0].String()},
	})
	return n
}

func (n *Node) Peers() []multiaddr.Multiaddr {
	res := n.Runner.MustRun(RunRequest{
		Path: n.IPFSBin,
		Args: []string{"swarm", "peers"},
	})
	lines := strings.Split(strings.TrimSpace(res.Stdout.String()), "\n")
	var addrs []multiaddr.Multiaddr
	for _, line := range lines {
		ma, err := multiaddr.NewMultiaddr(line)
		if err != nil {
			n.Log.Fatal(err)
		}
		addrs = append(addrs, ma)
	}
	return addrs
}

// GatewayURL waits for the gateway file and then returns its contents or times out.
func (n *Node) GatewayURL() string {
	timer := time.NewTimer(1 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			n.Log.Fatal("timeout waiting for gateway file")
		default:
			b, err := os.ReadFile(filepath.Join(n.Dir, "gateway"))
			if err == nil {
				return strings.TrimSpace(string(b))
			}
			if !errors.Is(err, fs.ErrNotExist) {
				n.Log.Fatal(err)
			}
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func (n *Node) GatewayClient() *HTTPClient {
	return &HTTPClient{
		Log:     n.Log,
		Client:  http.DefaultClient,
		BaseURL: n.GatewayURL(),
	}
}

func (n *Node) APIClient() *HTTPClient {
	return &HTTPClient{
		Log:     n.Log,
		Client:  http.DefaultClient,
		BaseURL: n.APIURL(),
	}
}
