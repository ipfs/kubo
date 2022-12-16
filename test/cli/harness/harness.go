package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/ipfs/kubo/test/cli/testutils"
)

// Harness tracks state for a test, such as temp dirs and IFPS nodes, and cleans them up after the test.
type Harness struct {
	Dir       string
	IPFSBin   string
	Runner    *Runner
	NodesRoot string
	Nodes     Nodes
	Log       *TestLogger
	T         *testing.T
}

// New constructs a harness that cleans up after the given test is done.
func New(t *testing.T, options ...func(h *Harness)) *Harness {
	log := NewTestLogger(t)
	h := &Harness{
		Runner: &Runner{Log: log, Env: osEnviron()},
		Log:    log,
		T:      t,
	}

	// walk up to find the root dir, from which we can locate the binary
	wd, err := os.Getwd()
	if err != nil {
		h.Log.Fatal(err)
	}
	goMod := FindUp("go.mod", wd)
	if goMod == "" {
		h.Log.Fatal("unable to find root dir")
	}
	rootDir := filepath.Dir(goMod)
	h.IPFSBin = filepath.Join(rootDir, "cmd", "ipfs", "ipfs")

	// setup working dir
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		h.Log.Fatalf("error creating temp dir: %s", err)
	}
	h.Dir = tmpDir
	h.Runner.Dir = h.Dir

	h.NodesRoot = filepath.Join(h.Dir, ".nodes")

	// apply any customizations
	// this should happen after all initialization
	for _, o := range options {
		o(h)
	}

	t.Cleanup(h.Cleanup)

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
	node := buildNode(h.Log, h.IPFSBin, h.NodesRoot, nodeID)
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
		h.Log.Fatalf("writing to temp file: %s", err.Error())
	}
	err = f.Close()
	if err != nil {
		h.Log.Fatalf("closing temp file: %s", err.Error())
	}
	return f.Name()
}

// TempFile creates a new unique temp file.
func (h *Harness) TempFile() *os.File {
	f, err := os.CreateTemp(h.Dir, "")
	if err != nil {
		h.Log.Fatalf("creating temp file: %s", err.Error())
	}
	return f
}

// WriteFile writes a file given a filename and its contents.
// The filename must be a relative path, or this panics.
func (h *Harness) WriteFile(filename, contents string) {
	if filepath.IsAbs(filename) {
		h.Log.Fatalf("%s must be a relative path", filename)
	}
	absPath := filepath.Join(h.Runner.Dir, filename)
	err := os.MkdirAll(filepath.Dir(absPath), 0777)
	if err != nil {
		h.Log.Fatalf("creating intermediate dirs for %q: %s", filename, err.Error())
	}
	err = os.WriteFile(absPath, []byte(contents), 0644)
	if err != nil {
		h.Log.Fatalf("writing %q (%q): %s", filename, absPath, err.Error())
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
			h.Log.Fatalf("%s must be a relative path when making dirs", path)
		}
		absPath := filepath.Join(h.Runner.Dir, path)
		err := os.MkdirAll(absPath, 0777)
		if err != nil {
			h.Log.Fatalf("recursively making dirs under %s: %s", absPath, err)
		}
	}
}

func (h *Harness) Sh(expr string) RunResult {
	return h.Runner.Run(RunRequest{
		Path: "bash",
		Args: []string{"-c", expr},
	})
}

func (h *Harness) Cleanup() {
	h.Log.Logf("cleaning up cluster")
	h.Nodes.StopDaemons()
	// TODO: don't do this if test fails, not sure how?
	h.Log.Logf("removing harness dir")
	err := os.RemoveAll(h.Dir)
	if err != nil {
		h.Log.Fatalf("removing temp dir %s: %s", h.Dir, err)
	}
}

func (h *Harness) EnableLogs() *Harness {
	h.Log.EnableLogs()
	return h
}
