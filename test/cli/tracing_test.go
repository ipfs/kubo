package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var otelCollectorConfigYAML = `
receivers:
  otlp:
    protocols:
      grpc:

processors:
  batch:

exporters:
  file:
    path: /traces/traces.json

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [file]
`

func TestTracing(t *testing.T) {
	testutils.RequiresDocker(t)
	t.Parallel()
	node := harness.NewT(t).NewNode().Init()

	node.WriteBytes("collector-config.yaml", []byte(otelCollectorConfigYAML))

	// touch traces.json and give it 777 perms in case Docker runs as a different user
	node.WriteBytes("traces.json", nil)
	err := os.Chmod(filepath.Join(node.Dir, "traces.json"), 0o777)
	require.NoError(t, err)

	dockerBin, err := exec.LookPath("docker")
	require.NoError(t, err)
	node.Runner.MustRun(harness.RunRequest{
		Path: dockerBin,
		Args: []string{
			"run",
			"--rm",
			"--detach",
			"--volume", fmt.Sprintf("%s:/config.yaml", filepath.Join(node.Dir, "collector-config.yaml")),
			"--volume", fmt.Sprintf("%s:/traces", node.Dir),
			"--net", "host",
			"--name", "ipfs-test-otel-collector",
			"otel/opentelemetry-collector-contrib:0.52.0",
			"--config", "/config.yaml",
		},
	})

	t.Cleanup(func() {
		node.Runner.MustRun(harness.RunRequest{
			Path: dockerBin,
			Args: []string{"stop", "ipfs-test-otel-collector"},
		})
	})

	node.Runner.Env["OTEL_TRACES_EXPORTER"] = "otlp"
	node.Runner.Env["OTEL_EXPORTER_OTLP_PROTOCOL"] = "grpc"
	node.Runner.Env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://localhost:4317"
	node.StartDaemon()

	assert.Eventually(t,
		func() bool {
			b, err := os.ReadFile(filepath.Join(node.Dir, "traces.json"))
			require.NoError(t, err)
			return strings.Contains(string(b), "go-ipfs")
		},
		5*time.Minute,
		10*time.Millisecond,
	)
}
