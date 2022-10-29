package harness

import (
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type IPTB struct {
	IPTBRoot string
	IPTBBin  string
	IPFSBin  string
	Runner   *Runner

	NumNodes int

	inited      bool
	nodeStarted map[int]bool
	mut         sync.Mutex
}

func (i *IPTB) Init(count int) {
	i.mut.Lock()
	defer i.mut.Unlock()
	if i.inited {
		panic("cannot init IPTB until it is stopped first")
	}

	i.NumNodes = count
	args := []string{"testbed", "create",
		"-type", "localipfs",
		"-count", strconv.Itoa(count),
		"-force",
		"-init",
	}
	i.Runner.MustRun(RunRequest{Path: i.IPTBBin, Args: args})
	i.nodeStarted = map[int]bool{}

	i.inited = true
}

func (i *IPTB) MustRun(args ...string) RunResult {
	return i.Runner.MustRun(RunRequest{Path: i.IPTBBin, Args: args})
}

func (i *IPTB) StartupCluster(args ...string) {
	i.mut.Lock()
	defer i.mut.Unlock()
	if !i.inited {
		panic("cannot start an IPTB cluster before it's inited")
	}

	bound := i.NumNodes - 1
	startArgs := []string{
		"start",
		"-wait",
		fmt.Sprintf("[0-%d]", bound),
	}

	if len(args) > 0 {
		startArgs = append(startArgs, "--")
	}
	startArgs = append(startArgs, args...)
	i.Runner.MustRun(RunRequest{Path: i.IPTBBin, Args: startArgs})

	i.Runner.MustRun(RunRequest{
		Path: i.IPTBBin,
		Args: []string{"connect", fmt.Sprintf("[1-%d]", bound), "0"},
	})

	for node := 0; node < i.NumNodes; node++ {
		res := i.MustRunIPFS(node, "swarm", "peers")
		stdout := res.Stdout.String()
		if !strings.Contains(stdout, "p2p") {
			log.Fatalf("unexpected state for node %d, stdout:\n%s", node, stdout)
		}
		i.nodeStarted[node] = true
	}
}

func (i *IPTB) StopNode(node int) {
	i.mut.Lock()
	defer i.mut.Unlock()
	if !i.inited {
		return
	}

	i.Runner.MustRun(RunRequest{
		Path: i.IPTBBin,
		Args: []string{"stop", strconv.Itoa(node)},
	})
	i.nodeStarted[node] = false
}

func (i *IPTB) Stop() {
	i.mut.Lock()
	defer i.mut.Unlock()
	if !i.inited {
		return
	}
	for node, started := range i.nodeStarted {
		if started {
			i.Runner.MustRun(RunRequest{
				Path: i.IPTBBin,
				Args: []string{"stop", strconv.Itoa(node)},
			})
		}
		i.nodeStarted[node] = false
	}
}

func (i *IPTB) PeerID(node int) string {
	res := i.MustRun("attr", "get", strconv.Itoa(node), "id")
	return strings.TrimSpace(res.Stdout.String())
}

func (i *IPTB) MustRunIPFS(node int, args ...string) RunResult {
	res := i.RunIPFS(node, args...)
	i.Runner.AssertNoError(res)
	return res
}

func (i *IPTB) RunIPFS(node int, args ...string) RunResult {
	return i.Runner.Run(RunRequest{
		Path:    i.IPFSBin,
		Args:    args,
		CmdOpts: []CmdOpt{i.Runner.RunWithEnv(map[string]string{"IPFS_PATH": i.IPFSPath(node)})},
	})
}

func (i *IPTB) IPFSPath(node int) string {
	return filepath.Join(i.IPTBRoot, "testbeds", "default", strconv.Itoa(node))
}
