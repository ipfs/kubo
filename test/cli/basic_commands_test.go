package cli

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/ipfs/go-ipfs/test/cli/harness"
	"github.com/stretchr/testify/assert"
	gomod "golang.org/x/mod/module"
)

var versionRegexp = regexp.MustCompile(`^ipfs version (.+)$`)

func parseVersionOutput(s string) semver.Version {
	versString := versionRegexp.FindStringSubmatch(s)[1]
	v, err := semver.Parse(versString)
	if err != nil {
		panic(err)
	}
	return v
}

func TestCurDirIsWritable(t *testing.T) {
	h := harness.NewForTest(t)
	h.WriteFile("test.txt", "It works!")
}

func TestIPFSVersionCommandMatchesFlag(t *testing.T) {
	t.Parallel()
	h := harness.NewForTest(t)
	commandVersionStr := h.MustRunIPFS("version").Stdout.String()
	commandVersionStr = strings.TrimSpace(commandVersionStr)
	commandVersion := parseVersionOutput(commandVersionStr)

	flagVersionStr := h.MustRunIPFS("--version").Stdout.String()
	flagVersionStr = strings.TrimSpace(flagVersionStr)
	flagVersion := parseVersionOutput(flagVersionStr)

	assert.Equal(t, commandVersion, flagVersion)
}

func TestIPFSVersionAll(t *testing.T) {
	t.Parallel()
	h := harness.NewForTest(t)
	res := h.MustRunIPFS("version", "--all").Stdout.String()
	res = strings.TrimSpace(res)
	assert.Contains(t, res, "go-ipfs version")
	assert.Contains(t, res, "Repo version")
	assert.Contains(t, res, "System version")
	assert.Contains(t, res, "Golang version")
}

func TestIPFSVersionDeps(t *testing.T) {
	t.Parallel()
	h := harness.NewForTest(t)
	res := h.MustRunIPFS("version", "deps").Stdout.String()
	res = strings.TrimSpace(res)
	lines := SplitLines(res)

	assert.Equal(t, "github.com/ipfs/go-ipfs@(devel)", lines[0])

	for _, depLine := range lines[1:] {
		split := strings.Split(depLine, " => ")
		for _, moduleVersion := range split {
			splitModVers := strings.Split(moduleVersion, "@")
			modPath := splitModVers[0]
			modVers := splitModVers[1]
			assert.NoError(t, gomod.Check(modPath, modVers))
		}
	}
}

func TestIPFSCommands(t *testing.T) {
	t.Parallel()
	h := harness.NewForTest(t)
	t.Cleanup(h.Cleanup)
	cmds := h.IPFSCommands()
	assert.Contains(t, cmds, "ipfs add")
	assert.Contains(t, cmds, "ipfs daemon")
	assert.Contains(t, cmds, "ipfs update")
}

func TestAllSubcommandsAcceptHelp(t *testing.T) {
	h := harness.NewForTest(t)
	wg := sync.WaitGroup{}
	for _, cmd := range h.IPFSCommands() {
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			splitCmd := strings.Split(cmd, " ")[1:]
			h.MustRunIPFS(StrConcat("help", splitCmd)...)
			h.MustRunIPFS(StrConcat(splitCmd, "--help")...)
		}(cmd)
	}
	wg.Wait()
}

func TestAllRootCommandsAreMentionedInHelpText(t *testing.T) {
	t.Parallel()
	h := harness.NewForTest(t)
	cmds := h.IPFSCommands()
	var rootCmds []string
	for _, cmd := range cmds {
		splitCmd := strings.Split(cmd, " ")
		if len(splitCmd) == 2 {
			rootCmds = append(rootCmds, splitCmd[1])
		}
	}

	// a few base commands are not expected to be in the help message
	// but we default to requiring them to be in the help message, so that we
	// have to make an conscious decision to exclude them
	notInHelp := map[string]bool{
		"object":   true,
		"shutdown": true,
		"tar":      true,
		"urlstore": true,
	}

	helpMsg := strings.TrimSpace(h.MustRunIPFS("--help").Stdout.String())
	for _, rootCmd := range rootCmds {
		if _, ok := notInHelp[rootCmd]; ok {
			continue
		}
		assert.Contains(t, helpMsg, fmt.Sprintf("  %s", rootCmd))
	}
}

func TestCommandDocsWidth(t *testing.T) {
	h := harness.NewForTest(t)

	// require new commands to explicitly opt in to longer lines
	allowList := map[string]bool{
		"ipfs add":                      true,
		"ipfs block put":                true,
		"ipfs daemon":                   true,
		"ipfs config profile":           true,
		"ipfs pin remote service":       true,
		"ipfs name pubsub":              true,
		"ipfs object patch":             true,
		"ipfs swarm connect":            true,
		"ipfs p2p forward":              true,
		"ipfs swarm disconnect":         true,
		"ipfs swarm addrs listen":       true,
		"ipfs dag resolve":              true,
		"ipfs object stat":              true,
		"ipfs pin remote add":           true,
		"ipfs config show":              true,
		"ipfs pin remote rm":            true,
		"ipfs dht get":                  true,
		"ipfs pin remote service add":   true,
		"ipfs file ls":                  true,
		"ipfs pin update":               true,
		"ipfs p2p":                      true,
		"ipfs resolve":                  true,
		"ipfs dag stat":                 true,
		"ipfs name publish":             true,
		"ipfs object diff":              true,
		"ipfs object patch add-link":    true,
		"ipfs name":                     true,
		"ipfs object patch append-data": true,
		"ipfs object patch set-data":    true,
		"ipfs dht put":                  true,
		"ipfs diag profile":             true,
		"ipfs swarm addrs local":        true,
		"ipfs files ls":                 true,
		"ipfs stats bw":                 true,
	}
	wg := sync.WaitGroup{}
	for _, cmd := range h.IPFSCommands() {
		if _, ok := allowList[cmd]; ok {
			continue
		}
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			splitCmd := strings.Split(cmd, " ")
			resStr := h.MustRunIPFS(StrConcat(splitCmd[1:], "--help")...)
			res := strings.TrimSpace(resStr.Stdout.String())
			for i, line := range harness.SplitLines(res) {
				assert.LessOrEqualf(t, len(line), 80, cmd, i)
			}
		}(cmd)
	}
	wg.Wait()
}

func TestAllCommandsFailWhenPassedBadFlag(t *testing.T) {
	h := harness.NewForTest(t)

	wg := sync.WaitGroup{}
	for _, cmd := range h.IPFSCommands() {
		wg.Add(1)
		go func(cmd string) {
			defer wg.Done()
			splitCmd := strings.Split(cmd, " ")
			res := h.Runner.Run(harness.RunRequest{
				Path: h.IPFSBin,
				Args: append(splitCmd, "--badflag"),
			})
			assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		}(cmd)
	}
	wg.Wait()
}

func TestCommandsFlags(t *testing.T) {
	h := harness.NewForTest(t)
	resStr := h.MustRunIPFS("commands", "--flags").Stdout.String()
	assert.Contains(t, resStr, "ipfs pin add --recursive / ipfs pin add -r")
	assert.Contains(t, resStr, "ipfs id --format / ipfs id -f")
	assert.Contains(t, resStr, "ipfs repo gc --quiet / ipfs repo gc -q")
}
