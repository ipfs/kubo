package main_test

import (
	"bufio"
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	sticonfig "github.com/filecoin-project/storetheindex/config"
	qt "github.com/frankban/quicktest"
	serialize "github.com/ipfs/go-ipfs/config/serialize"
	randomfiles "github.com/jbenet/go-random-files"
	peer "github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

// This is a full end-to-end test with storetheindex as the indexer daemon,
// and go-ipfs as a client.
// We build both programs, noting that we always build the latest go-ipfs idxprovider branch.
// We initialize their setup, start the two daemons, and connect the peers.
// We then add some randomized content and query its CIDs on the indexer.

type e2eTestRunner struct {
	t   *testing.T
	dir string
	ctx context.Context
	env []string

	indexerReady                 chan struct{}
	goIPFSReady                  chan struct{}
	indexerHasRegisteredProvider chan struct{}
}

func (e *e2eTestRunner) run(name string, args ...string) []byte {
	e.t.Helper()

	e.t.Logf("run: %s %s", name, strings.Join(args, " "))

	cmd := exec.CommandContext(e.ctx, name, args...)
	cmd.Env = e.env
	out, err := cmd.CombinedOutput()
	qt.Assert(e.t, err, qt.IsNil, qt.Commentf("output: %s", out))
	return out
}

func (e *e2eTestRunner) start(prog string, args ...string) *exec.Cmd {
	e.t.Helper()

	name := filepath.Base(prog)
	e.t.Logf("run: %s %s", name, strings.Join(args, " "))

	cmd := exec.CommandContext(e.ctx, prog, args...)
	cmd.Env = e.env
	switch name {
	case "storetheindex":
		cmd.Env = append(cmd.Env, "GOLOG_LOG_LEVEL=info")
	case "ipfs":
		cmd.Env = append(cmd.Env, "GOLOG_LOG_LEVEL=idxProvider=debug,graphsync=debug,graphsync_network=debug")
	}

	stdout, err := cmd.StdoutPipe()
	qt.Assert(e.t, err, qt.IsNil)
	cmd.Stderr = cmd.Stdout

	scanner := bufio.NewScanner(stdout)

	go func() {
		closedRegister := false
		for scanner.Scan() {
			line := scanner.Text()

			// Logging every single line via the test output is verbose,
			// but helps see what's happening, especially when the test fails.
			e.t.Logf("%s: %s", name, line)

			switch name {
			case "storetheindex":
				if strings.Contains(line, "Indexer is ready") {
					close(e.indexerReady)
				} else if strings.Contains(line, "registered provider") {
					if !closedRegister {
						close(e.indexerHasRegisteredProvider)
						closedRegister = true
					}
				}
			case "ipfs":
				line = strings.ToLower(line)
				if strings.Contains(line, "daemon is ready") {
					close(e.goIPFSReady)
				}
			}
		}
	}()

	err = cmd.Start()
	qt.Assert(e.t, err, qt.IsNil)
	return cmd
}

func (e *e2eTestRunner) stop(cmd *exec.Cmd, timeout time.Duration) {
	sig := os.Interrupt
	if runtime.GOOS == "windows" {
		// Windows can't send SIGINT.
		sig = os.Kill
	}
	err := cmd.Process.Signal(sig)
	qt.Assert(e.t, err, qt.IsNil)

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	select {
	case <-time.After(timeout):
		e.t.Logf("killing command after %s: %s", timeout, cmd)
		err := cmd.Process.Kill()
		qt.Assert(e.t, err, qt.IsNil)
	case err := <-waitErr:
		qt.Assert(e.t, err, qt.IsNil)
	}
}

func TestEndToEndWithGoIPFSFullImport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	e := &e2eTestRunner{
		t:   t,
		dir: t.TempDir(),
		ctx: ctx,

		indexerReady:                 make(chan struct{}),
		goIPFSReady:                  make(chan struct{}),
		indexerHasRegisteredProvider: make(chan struct{}),
	}

	setupEnvironment(e)

	goIPFS := filepath.Join(e.dir, "ipfs")
	// install from our cloned folder the ipfs binary into the tempdir (GOBIN)
	e.run("go", "install", ".")

	cwd, err := os.Getwd()
	qt.Assert(t, err, qt.IsNil)

	indexer, indexerID := setupSTH(e)

	err = os.Chdir(cwd)
	qt.Assert(t, err, qt.IsNil)

	e.run(goIPFS, "init")

	ip, err := getOutboundIP()
	qt.Assert(t, err, qt.IsNil)

	peerID, ipaddr := configureIPFS(e, indexerID, ip)

	cmdIndexer, cmdProvider := startupDaemons(e, indexer, ctx, goIPFS, ipaddr)

	// get the local refs, pull the first one and make sure it gets to indexer
	refsout := e.run(goIPFS, "refs", "local")
	t.Logf("Refs local: %s", refsout)

	// TODO - without doing this `second` advertisement the first one never seems
	// to get acted upon - why?

	// Add content to go-ipfs.  This will cause the index provider of go-ipfs to
	// publish an advertisement that the indexer will read.  The indexer will
	// then import the advertised content.
	tmpDir := createTempFiles(e.dir, t, 200, 256)
	addoutput := e.run(goIPFS, "add", tmpDir, "-r")
	t.Logf("add output: %s", addoutput)

	t.Log("Sleeping so indexer can do it's thing")
	time.Sleep(time.Minute * 3)

	provider := installProvider(e)
	err = os.Chdir(cwd)
	qt.Assert(t, err, qt.IsNil)

	verifyoutput := e.run(provider, "verify-ingest", "--from-provider", "/ip4/"+ip.String()+"/tcp/4001/p2p/"+peerID, "--to", ip.String()+":3000", "--pid", peerID)
	t.Logf("verify output: %s", verifyoutput)
	if bytes.Contains(verifyoutput, []byte("Passed verification check.")) {
		t.Log("Verification passed")
	} else if !bytes.Contains(verifyoutput, []byte("Failed verification check.")) {
		t.Fatal("Failed verification check")
	} else {
		t.Fatal("Unable to determine pass or fail")
	}

	e.stop(cmdIndexer, time.Second)
	e.stop(cmdProvider, time.Second)
}

func TestEndToEndWithGoIPFSOngoingImport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on windows")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	e := &e2eTestRunner{
		t:   t,
		dir: t.TempDir(),
		ctx: ctx,

		indexerReady:                 make(chan struct{}),
		goIPFSReady:                  make(chan struct{}),
		indexerHasRegisteredProvider: make(chan struct{}),
	}

	setupEnvironment(e)

	goIPFS := filepath.Join(e.dir, "ipfs")
	// install from our cloned folder the ipfs binary into the tempdir (GOBIN)
	e.run("go", "install", ".")

	indexer, indexerID := setupSTH(e)
	e.run(goIPFS, "init")

	ip, err := getOutboundIP()
	qt.Assert(t, err, qt.IsNil)

	peerID, ipaddr := configureIPFS(e, indexerID, ip)

	cmdIndexer, cmdProvider := startupDaemons(e, indexer, ctx, goIPFS, ipaddr)

	// Add content to go-ipfs.  This will cause the index provider of go-ipfs to
	// publish an advertisement that the indexer will read.  The indexer will
	// then import the advertised content.
	tmpDir := createTempFiles(e.dir, t, 500, 1024)
	addoutput := e.run(goIPFS, "add", tmpDir, "-r")
	t.Logf("add output: %s", addoutput)
	// addedCid := parseCid(addoutput, t)

	t.Log("Sleeping so indexer can do it's thing")
	time.Sleep(time.Minute * 3)

	provider := installProvider(e)

	verifyoutput := e.run(provider, "verify-ingest", "--from-provider", "/ip4/127.0.0.1/tcp/4001/p2p/"+peerID, "--to", "127.0.0.1:3000", "--pid", peerID)
	t.Logf("verify output: %s", verifyoutput)
	if bytes.Contains(verifyoutput, []byte("Passed verification check.")) {
		t.Log("Verification passed")
	} else if !bytes.Contains(verifyoutput, []byte("Failed verification check.")) {
		t.Fatal("Failed verification check")
	} else {
		t.Fatal("Unable to determine pass or fail")
	}

	e.stop(cmdIndexer, time.Second)
	e.stop(cmdProvider, time.Second)
}

// Get preferred outbound ip of this machine
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}

func createTempFiles(root string, t *testing.T, numfiles int, maxfilesize int) string {
	// add a bunch of temporary files
	opts := randomfiles.Options{
		FileSize:    maxfilesize,
		RandomSize:  true,
		Alphabet:    randomfiles.RunesEasy,
		FanoutFiles: numfiles,
		Source:      crand.Reader,
	}
	tmpDir := filepath.Join(root, "tmpTestDir")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatalf("Error creating random files root directory: %s", err)
	}
	err := randomfiles.WriteRandomFiles(tmpDir, 1, &opts)
	if err != nil {
		t.Fatalf("Error creating random files: %s", err)
	}
	return tmpDir
}

func installProvider(e *e2eTestRunner) string {
	provider := filepath.Join(e.dir, "provider")

	err := os.Chdir(e.dir)
	qt.Assert(e.t, err, qt.IsNil)
	e.run("git", "clone", "https://github.com/filecoin-project/index-provider.git", "ip")
	err = os.Chdir("ip")
	qt.Assert(e.t, err, qt.IsNil)
	err = os.Chdir("cmd")
	qt.Assert(e.t, err, qt.IsNil)
	e.run("go", "install", "./provider")
	return provider
}

func setupEnvironment(e *e2eTestRunner) {
	// Use a clean environment, with the host's PATH, and a temporary HOME.
	// We also tell "go install" to place binaries there.
	hostEnv := os.Environ()
	var filteredEnv []string
	for _, env := range hostEnv {
		if strings.Contains(env, "CC") || strings.Contains(env, "LDFLAGS") || strings.Contains(env, "CFLAGS") {
			// Bring in the C compiler flags from the host. For example on a Nix
			// machine, this compilation within the test will fail since the compiler
			// will not find correct libraries.
			filteredEnv = append(filteredEnv, env)
		} else if strings.HasPrefix(env, "PATH") {
			// Bring in the host's PATH.
			filteredEnv = append(filteredEnv, env)
		}
	}
	e.env = append(filteredEnv, []string{
		"HOME=" + e.dir,
		"GOBIN=" + e.dir,
	}...)
	e.t.Logf("Env: %s", strings.Join(e.env, " "))

	// Reuse the host's build and module download cache.
	// This should allow "go install" to reuse work.
	for _, name := range []string{"GOCACHE", "GOMODCACHE"} {
		out, err := exec.Command("go", "env", name).CombinedOutput()
		qt.Assert(e.t, err, qt.IsNil)
		out = bytes.TrimSpace(out)
		e.env = append(e.env, fmt.Sprintf("%s=%s", name, out))
	}
}

func setupSTH(e *e2eTestRunner) (string, string) {
	indexer := filepath.Join(e.dir, "storetheindex")

	err := os.Chdir(e.dir)
	qt.Assert(e.t, err, qt.IsNil)
	e.run("git", "clone", "https://github.com/filecoin-project/storetheindex.git", "sth")
	err = os.Chdir("sth")
	qt.Assert(e.t, err, qt.IsNil)
	// install from our tempdir/sth the storetheindex binary into the tempdir (GOBIN)
	e.run("go", "install")

	e.run(indexer, "init")

	stipath := filepath.Join(e.dir, ".storetheindex", "config")
	sticfg, err := sticonfig.Load(stipath)
	qt.Assert(e.t, err, qt.IsNil)
	indexerID := sticfg.Identity.PeerID
	sticfg.Discovery.PollInterval = sticonfig.Duration(time.Minute * 1)
	err = sticfg.Save(stipath)
	qt.Assert(e.t, err, qt.IsNil)
	return indexer, indexerID
}

func configureIPFS(e *e2eTestRunner, indexerID string, ip net.IP) (string, string) {
	// add storetheindex as a peer in config file
	cfgFile := filepath.Join(e.dir, ".ipfs", "config")

	// read in ipfs config, add peer info, write it back out
	cfgTmp, err := serialize.Load(cfgFile)
	// providerID := cfgTmp.Identity.PeerID
	// t.Logf("Initialized go-ipfs provider ID: %s", providerID)

	qt.Assert(e.t, err, qt.IsNil)
	cfgMod, err := cfgTmp.Clone()
	qt.Assert(e.t, err, qt.IsNil)
	ipaddr := "/ip4/" + ip.String() + "/tcp/3003"
	addr, err := ma.NewMultiaddr(ipaddr)
	qt.Assert(e.t, err, qt.IsNil)
	addr2, err := ma.NewMultiaddr("/ip4/127.0.0.1/tcp/3003")
	qt.Assert(e.t, err, qt.IsNil)
	id, err := peer.Decode(indexerID)
	qt.Assert(e.t, err, qt.IsNil)
	peers := make([]peer.AddrInfo, 1)
	peers[0] = peer.AddrInfo{
		ID:    id,
		Addrs: []ma.Multiaddr{addr, addr2},
	}
	cfgMod.Peering.Peers = peers
	peerID := cfgMod.Identity.PeerID

	err = serialize.WriteConfigFile(cfgFile, cfgMod)
	qt.Assert(e.t, err, qt.IsNil)

	return peerID, ipaddr
}

func startupDaemons(e *e2eTestRunner, indexer string, ctx context.Context, goIPFS string, ipaddr string) (*exec.Cmd, *exec.Cmd) {
	cmdIndexer := e.start(indexer, "daemon")
	select {
	case <-e.indexerReady:
	case <-ctx.Done():
		e.t.Fatal("timed out waiting for indexer to start")
	}

	cmdProvider := e.start(goIPFS, "daemon", "--index-provider-experiment", ipaddr, "--enable-pubsub-experiment")
	select {
	case <-e.goIPFSReady:
	case <-ctx.Done():
		e.t.Fatal("timed out waiting for go-ipfs idx provider to start")
	}

	select {
	case <-e.indexerHasRegisteredProvider:
	case <-ctx.Done():
		e.t.Fatal("timed out waiting for go-ipfs to register to indexer")
	}
	return cmdIndexer, cmdProvider
}
