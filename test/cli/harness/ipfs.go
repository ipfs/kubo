package harness

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"reflect"
	"strings"
)

func (h *Harness) MustRunIPFS(args ...string) RunResult {
	return h.Runner.MustRun(RunRequest{Path: h.IPFSBin, Args: args})
}

func (h *Harness) IPFSCommands() []string {
	res := h.MustRunIPFS("commands").Stdout.String()
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

func (h *Harness) Init() {
	h.MustRunIPFS("init", "--profile=test")
	h.Mkdirs("mountdir", "ipfs", "ipns")
	h.SetIPFSConfig("Mounts.IPFS", h.IPFSMountpoint)
	h.SetIPFSConfig("Mounts.IPNS", h.IPNSMountpoint)

	configPath := filepath.Join(h.IPFSPath, "config")
	b, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("reading config file from %s: %s", configPath, err)
	}
	if len(b) == 0 {
		log.Fatalf("expected non-empty config at %s", configPath)
	}
}

func (h *Harness) SetIPFSConfig(key string, val interface{}, flags ...string) {
	valBytes, err := json.Marshal(val)
	if err != nil {
		log.Fatalf("marshling config for key '%s': %s", key, err)
	}
	valStr := string(valBytes)

	args := []string{"config", "--json"}
	args = append(args, flags...)
	args = append(args, key, valStr)
	h.MustRunIPFS(args...)

	// validate the config was set correctly
	var newVal string
	h.GetIPFSConfig(key, &newVal)
	if val != newVal {
		log.Fatalf("key '%s' did not retain value '%s' after it was set, got '%s'", key, val, newVal)
	}
}

func (h *Harness) GetIPFSConfig(key string, val interface{}) {
	res := h.MustRunIPFS("config", key)
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
	return
}

func (h *Harness) IPFSAdd(content io.Reader, args ...string) string {
	fullArgs := []string{"add", "-q"}
	fullArgs = append(fullArgs, args...)
	res := h.Runner.MustRun(RunRequest{
		Path:    h.IPFSBin,
		Args:    fullArgs,
		CmdOpts: []CmdOpt{h.Runner.RunWithStdin(content)},
	})
	return strings.TrimSpace(res.Stdout.String())
}
