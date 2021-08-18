package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Harness is used within the context of a single test, setting up the test environment, tracking state, and cleaning up.
type Harness struct {
	T *testing.T

	IPFSBin        string
	Expensive      bool
	RequiresFuse   bool
	RequiresDocker bool
	RequiresPlugin bool

	skip   bool
	tmpDir string
}

func New(t *testing.T, options ...func(h *Harness)) *Harness {
	h := &Harness{
		T: t,
	}

	relPath := filepath.FromSlash("../../cmd/ipfs/ipfs")
	absPath, err := filepath.Abs(relPath)
	if err != nil {
		panic(fmt.Sprintf("unable to find absolute path of %s: %s", relPath, err))
	}
	h.IPFSBin = absPath

	for _, o := range options {
		o(h)
	}
	if os.Getenv("TEST_NO_DOCKER") == "1" && h.RequiresDocker {
		h.T.SkipNow()
	}
	if os.Getenv("TEST_NO_FUSE") == "1" && h.RequiresFuse {
		h.T.SkipNow()
	}
	if (os.Getenv("TEST_EXPENSIVE") == "1" && !h.Expensive) || testing.Short() {
		h.T.SkipNow()
	}
	if os.Getenv("TEST_NO_PLUGIN") == "1" && h.RequiresPlugin {
		h.T.SkipNow()
	}

	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		panic(fmt.Sprintf("error creating temp dir: %s", err))
	}
	h.tmpDir = tmpDir

	t.Cleanup(h.Cleanup)

	return h
}

type RunResult struct {
	Stdout  *bytes.Buffer
	Stderr  *bytes.Buffer
	Err     error
	ExitErr *exec.ExitError
	Cmd     *exec.Cmd
}

func (h *Harness) IPFS(args ...string) *RunResult {
	return h.Run(h.IPFSBin, args...)
}

func (h *Harness) Run(cmdName string, args ...string) *RunResult {
	h.T.Helper()
	return h.RunOpts(cmdName, args)
}

// Run a command and return the result.
// The options are applied just before the command is run.
// Fails the test if the command fails.
func (h *Harness) RunOpts(cmdName string, args []string, opts ...func(*exec.Cmd)) *RunResult {
	cmd := exec.Command(cmdName, args...)
	stdoutBuf := bytes.Buffer{}
	stderrBuf := bytes.Buffer{}
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	for _, o := range opts {
		o(cmd)
	}

	h.T.Logf("running: '%s', args: '%v'", cmdName, args)

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			h.T.Logf("%s returned error when testing completion, code: %d, err: %s", cmdName, exitErr.ExitCode(), exitErr.Error())
			h.T.Log("stdout:", stdoutBuf.String())
			h.T.Log("stderr:", stderrBuf.String())
			h.T.FailNow()
		} else {
			h.T.Fatalf("unable to run %s: %s", cmdName, err)
		}
	}
	return &RunResult{
		Stdout: &stdoutBuf,
		Stderr: &stderrBuf,
		Cmd:    cmd,
	}
}

func (h *Harness) Sh(expr string) *RunResult {
	return h.Run("bash", "-c", expr)
}

func (h *Harness) WriteToTemp(contents string) string {
	f, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	f.WriteString(contents)
	err = f.Close()
	if err != nil {
		panic(err)
	}
	return f.Name()
}

func (h *Harness) Cleanup() {
	h.T.Log("cleaning up test")
	err := os.RemoveAll(h.tmpDir)
	if err != nil {
		panic(fmt.Sprintf("error removing temp dir: %s", err))
	}
}
