package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	. "github.com/ipfs/kubo/test/cli/testutils"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Harness tracks state for a test, such as temp dirs and IFPS nodes, and cleans them up after the test.
type Harness struct {
	Dir       string
	IPFSBin   string
	Runner    *Runner
	NodesRoot string
	Nodes     Nodes
}

// TODO: use zaptest.NewLogger(t) instead
func EnableDebugLogging() {
	err := logging.SetLogLevel("testharness", "DEBUG")
	if err != nil {
		panic(err)
	}
}

// NewT constructs a harness that cleans up after the given test is done.
func NewT(t *testing.T, options ...func(h *Harness)) *Harness {
	h := New(options...)
	t.Cleanup(h.Cleanup)
	return h
}

func New(options ...func(h *Harness)) *Harness {
	h := &Harness{Runner: &Runner{Env: osEnviron()}}

	// walk up to find the root dir, from which we can locate the binary
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	goMod := FindUp("go.mod", wd)
	if goMod == "" {
		panic("unable to find root dir")
	}
	rootDir := filepath.Dir(goMod)
	h.IPFSBin = filepath.Join(rootDir, "cmd", "ipfs", "ipfs")

	// setup working dir
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		log.Panicf("error creating temp dir: %s", err)
	}
	h.Dir = tmpDir
	h.Runner.Dir = h.Dir

	h.NodesRoot = filepath.Join(h.Dir, ".nodes")

	// apply any customizations
	// this should happen after all initialization
	for _, o := range options {
		o(h)
	}

	return h
}

func osEnviron() map[string]string {
	m := map[string]string{}
	for _, entry := range os.Environ() {
		split := strings.Split(entry, "=")
		m[split[0]] = split[1]
	}
	return m
}

func (h *Harness) NewNode() *Node {
	nodeID := len(h.Nodes)
	node := BuildNode(h.IPFSBin, h.NodesRoot, nodeID)
	h.Nodes = append(h.Nodes, node)
	return node
}

func (h *Harness) NewNodes(count int) Nodes {
	var newNodes []*Node
	for i := 0; i < count; i++ {
		newNodes = append(newNodes, h.NewNode())
	}
	return newNodes
}

// WriteToTemp writes the given contents to a guaranteed-unique temp file, returning its path.
func (h *Harness) WriteToTemp(contents string) string {
	f := h.TempFile()
	_, err := f.WriteString(contents)
	if err != nil {
		log.Panicf("writing to temp file: %s", err.Error())
	}
	err = f.Close()
	if err != nil {
		log.Panicf("closing temp file: %s", err.Error())
	}
	return f.Name()
}

// TempFile creates a new unique temp file.
func (h *Harness) TempFile() *os.File {
	f, err := os.CreateTemp(h.Dir, "")
	if err != nil {
		log.Panicf("creating temp file: %s", err.Error())
	}
	return f
}

// WriteFile writes a file given a filename and its contents.
// The filename must be a relative path, or this panics.
func (h *Harness) WriteFile(filename, contents string) {
	if filepath.IsAbs(filename) {
		log.Panicf("%s must be a relative path", filename)
	}
	absPath := filepath.Join(h.Runner.Dir, filename)
	err := os.MkdirAll(filepath.Dir(absPath), 0o777)
	if err != nil {
		log.Panicf("creating intermediate dirs for %q: %s", filename, err.Error())
	}
	err = os.WriteFile(absPath, []byte(contents), 0o644)
	if err != nil {
		log.Panicf("writing %q (%q): %s", filename, absPath, err.Error())
	}
}

func WaitForFile(path string, timeout time.Duration) error {
	start := time.Now()
	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(1 * time.Millisecond)
	defer timer.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			return fmt.Errorf("timeout waiting for %s after %v", path, time.Since(start))
		case <-ticker.C:
			_, err := os.Stat(path)
			if err == nil {
				return nil
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("error waiting for %s: %w", path, err)
		}
	}
}

func (h *Harness) Mkdirs(paths ...string) {
	for _, path := range paths {
		if filepath.IsAbs(path) {
			log.Panicf("%s must be a relative path when making dirs", path)
		}
		absPath := filepath.Join(h.Runner.Dir, path)
		err := os.MkdirAll(absPath, 0o777)
		if err != nil {
			log.Panicf("recursively making dirs under %s: %s", absPath, err)
		}
	}
}

func (h *Harness) Sh(expr string) *RunResult {
	return h.Runner.Run(RunRequest{
		Path: "bash",
		Args: []string{"-c", expr},
	})
}

func (h *Harness) Cleanup() {
	log.Debugf("cleaning up cluster")
	h.Nodes.StopDaemons()
	// TODO: don't do this if test fails, not sure how?
	log.Debugf("removing harness dir")
	err := os.RemoveAll(h.Dir)
	if err != nil {
		log.Panicf("removing temp dir %s: %s", h.Dir, err)
	}
}

// ExtractPeerID extracts a peer ID from the given multiaddr, and fatals if it does not contain a peer ID.
func (h *Harness) ExtractPeerID(m multiaddr.Multiaddr) peer.ID {
	var peerIDStr string
	multiaddr.ForEach(m, func(c multiaddr.Component) bool {
		if c.Protocol().Code == multiaddr.P_P2P {
			peerIDStr = c.Value()
		}
		return true
	})
	if peerIDStr == "" {
		panic(multiaddr.ErrProtocolNotFound)
	}
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		panic(err)
	}
	return peerID
}
