package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	. "github.com/ipfs/kubo/test/cli/testutils"
)

func (n *Node) IPFSCommands() []string {
	res := n.IPFS("commands").Stdout.String()
	res = strings.TrimSpace(res)
	split := SplitLines(res)
	var cmds []string
	for _, line := range split {
		trimmed := strings.TrimSpace(line)
		if trimmed == "ipfs" {
			continue
		}
		cmds = append(cmds, trimmed)
	}
	return cmds
}

func (n *Node) SetIPFSConfig(key string, val interface{}, flags ...string) {
	valBytes, err := json.Marshal(val)
	if err != nil {
		log.Panicf("marshling config for key '%s': %s", key, err)
	}
	valStr := string(valBytes)

	args := []string{"config", "--json"}
	args = append(args, flags...)
	args = append(args, key, valStr)
	n.IPFS(args...)

	// validate the config was set correctly
	var newVal string
	n.GetIPFSConfig(key, &newVal)
	if val != newVal {
		log.Panicf("key '%s' did not retain value '%s' after it was set, got '%s'", key, val, newVal)
	}
}

func (n *Node) GetIPFSConfig(key string, val interface{}) {
	res := n.IPFS("config", key)
	valStr := strings.TrimSpace(res.Stdout.String())
	// only when the result is a string is the result not well-formed JSON,
	// so check the value type and add quotes if it's expected to be a string
	reflectVal := reflect.ValueOf(val)
	if reflectVal.Kind() == reflect.Ptr && reflectVal.Elem().Kind() == reflect.String {
		valStr = fmt.Sprintf(`"%s"`, valStr)
	}
	err := json.Unmarshal([]byte(valStr), val)
	if err != nil {
		log.Fatalf("unmarshaling config for key '%s', value '%s': %s", key, valStr, err)
	}
}

func (n *Node) IPFSAddStr(content string, args ...string) string {
	log.Debugf("node %d adding content '%s' with args: %v", n.ID, PreviewStr(content), args)
	return n.IPFSAdd(strings.NewReader(content), args...)
}

func (n *Node) IPFSAdd(content io.Reader, args ...string) string {
	log.Debugf("node %d adding with args: %v", n.ID, args)
	fullArgs := []string{"add", "-q"}
	fullArgs = append(fullArgs, args...)
	res := n.Runner.MustRun(RunRequest{
		Path:    n.IPFSBin,
		Args:    fullArgs,
		CmdOpts: []CmdOpt{RunWithStdin(content)},
	})
	out := strings.TrimSpace(res.Stdout.String())
	log.Debugf("add result: %q", out)
	return out
}
