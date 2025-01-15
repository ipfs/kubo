package cli

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/ipfs/kubo/test/cli/harness"
	. "github.com/ipfs/kubo/test/cli/testutils"
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
	t.Parallel()
	h := harness.NewT(t)
	h.WriteFile("test.txt", "It works!")
}

func TestIPFSVersionCommandMatchesFlag(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	commandVersionStr := node.IPFS("version").Stdout.String()
	commandVersionStr = strings.TrimSpace(commandVersionStr)
	commandVersion := parseVersionOutput(commandVersionStr)

	flagVersionStr := node.IPFS("--version").Stdout.String()
	flagVersionStr = strings.TrimSpace(flagVersionStr)
	flagVersion := parseVersionOutput(flagVersionStr)

	assert.Equal(t, commandVersion, flagVersion)
}

func TestIPFSVersionAll(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	res := node.IPFS("version", "--all").Stdout.String()
	res = strings.TrimSpace(res)
	assert.Contains(t, res, "Kubo version")
	assert.Contains(t, res, "Repo version")
	assert.Contains(t, res, "System version")
	assert.Contains(t, res, "Golang version")
}

func TestIPFSVersionDeps(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	res := node.IPFS("version", "deps").Stdout.String()
	res = strings.TrimSpace(res)
	lines := SplitLines(res)

	assert.Equal(t, "github.com/ipfs/kubo@(devel)", lines[0])

	for _, depLine := range lines[1:] {
		split := strings.Split(depLine, " => ")
		for _, moduleVersion := range split {
			splitModVers := strings.Split(moduleVersion, "@")
			modPath := splitModVers[0]
			modVers := splitModVers[1]
			assert.NoError(t, gomod.Check(modPath, modVers), "path: %s, version: %s", modPath, modVers)
		}
	}
}

func TestIPFSCommands(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	cmds := node.IPFSCommands()
	assert.Contains(t, cmds, "ipfs add")
	assert.Contains(t, cmds, "ipfs daemon")
	assert.Contains(t, cmds, "ipfs update")
}

func TestAllSubcommandsAcceptHelp(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	for _, cmd := range node.IPFSCommands() {
		cmd := cmd
		t.Run(fmt.Sprintf("command %q accepts help", cmd), func(t *testing.T) {
			t.Parallel()
			splitCmd := strings.Split(cmd, " ")[1:]
			node.IPFS(StrCat("help", splitCmd)...)
			node.IPFS(StrCat(splitCmd, "--help")...)
		})
	}
}

func TestAllRootCommandsAreMentionedInHelpText(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	cmds := node.IPFSCommands()
	var rootCmds []string
	for _, cmd := range cmds {
		splitCmd := strings.Split(cmd, " ")
		if len(splitCmd) == 2 {
			rootCmds = append(rootCmds, splitCmd[1])
		}
	}

	// a few base commands are not expected to be in the help message
	// but we default to requiring them to be in the help message, so that we
	// have to make a conscious decision to exclude them
	notInHelp := map[string]bool{
		"object":   true,
		"shutdown": true,
	}

	helpMsg := strings.TrimSpace(node.IPFS("--help").Stdout.String())
	for _, rootCmd := range rootCmds {
		if _, ok := notInHelp[rootCmd]; ok {
			continue
		}
		assert.Contains(t, helpMsg, fmt.Sprintf("  %s", rootCmd))
	}
}

func TestCommandDocsWidth(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

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
		"ipfs p2p close":                true,
		"ipfs swarm disconnect":         true,
		"ipfs swarm addrs listen":       true,
		"ipfs dag resolve":              true,
		"ipfs dag get":                  true,
		"ipfs pin remote add":           true,
		"ipfs config show":              true,
		"ipfs config edit":              true,
		"ipfs pin remote rm":            true,
		"ipfs pin remote ls":            true,
		"ipfs pin verify":               true,
		"ipfs pin remote service add":   true,
		"ipfs pin update":               true,
		"ipfs pin rm":                   true,
		"ipfs p2p":                      true,
		"ipfs resolve":                  true,
		"ipfs dag stat":                 true,
		"ipfs name publish":             true,
		"ipfs object diff":              true,
		"ipfs object patch add-link":    true,
		"ipfs name":                     true,
		"ipfs diag profile":             true,
		"ipfs diag cmds":                true,
		"ipfs swarm addrs local":        true,
		"ipfs files ls":                 true,
		"ipfs stats bw":                 true,
		"ipfs swarm peers":              true,
		"ipfs pubsub sub":               true,
		"ipfs files write":              true,
		"ipfs swarm limit":              true,
		"ipfs commands completion fish": true,
		"ipfs key export":               true,
		"ipfs routing get":              true,
		"ipfs refs":                     true,
		"ipfs refs local":               true,
		"ipfs cid base32":               true,
		"ipfs pubsub pub":               true,
		"ipfs repo ls":                  true,
		"ipfs routing put":              true,
		"ipfs key import":               true,
		"ipfs swarm peering add":        true,
		"ipfs swarm peering rm":         true,
		"ipfs swarm peering ls":         true,
		"ipfs update":                   true,
		"ipfs swarm stats":              true,
	}
	for _, cmd := range node.IPFSCommands() {
		if _, ok := allowList[cmd]; ok {
			continue
		}
		t.Run(fmt.Sprintf("command %q conforms to docs width limit", cmd), func(t *testing.T) {
			splitCmd := strings.Split(cmd, " ")
			resStr := node.IPFS(StrCat(splitCmd[1:], "--help")...)
			res := strings.TrimSpace(resStr.Stdout.String())
			for _, line := range SplitLines(res) {
				assert.LessOrEqualf(t, len(line), 80, "expected width %d < 80 for %q", len(line), cmd)
			}
		})
	}
}

func TestAllCommandsFailWhenPassedBadFlag(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()

	for _, cmd := range node.IPFSCommands() {
		t.Run(fmt.Sprintf("command %q fails when passed a bad flag", cmd), func(t *testing.T) {
			splitCmd := strings.Split(cmd, " ")
			res := node.RunIPFS(StrCat(splitCmd, "--badflag")...)
			assert.Equal(t, 1, res.Cmd.ProcessState.ExitCode())
		})
	}
}

func TestCommandsFlags(t *testing.T) {
	t.Parallel()
	node := harness.NewT(t).NewNode()
	resStr := node.IPFS("commands", "--flags").Stdout.String()
	assert.Contains(t, resStr, "ipfs pin add --recursive / ipfs pin add -r")
	assert.Contains(t, resStr, "ipfs id --format / ipfs id -f")
	assert.Contains(t, resStr, "ipfs repo gc --quiet / ipfs repo gc -q")
}
