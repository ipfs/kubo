package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	logging "github.com/ipfs/go-log/v2"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/node/libp2p"
	"github.com/ipfs/kubo/plugin"
	"github.com/ipfs/kubo/plugin/plugins/pebbleds"
	"github.com/ipfs/kubo/repo/fsrepo"
)

func mockServer(t *testing.T) (*httptest.Server, func() LogEvent) {
	t.Helper()

	var e LogEvent

	// Create a mock HTTP test server
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is POST to the correct endpoint
		if r.Method != "POST" || r.URL.Path != "/" {
			t.Log("invalid request")
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		// Check content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Log("invalid content type")
			http.Error(w, "invalid content type", http.StatusBadRequest)
			return
		}

		// Check if the body is not empty
		if r.Body == nil {
			t.Log("empty body")
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		// Read the body
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			t.Log("zero-length body")
			http.Error(w, "empty body", http.StatusBadRequest)
			return
		}

		t.Logf("Received telemetry:\n %s", string(body))

		err := json.Unmarshal(body, &e)
		if err != nil {
			t.Log("error unmarshaling event", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Return success
		w.WriteHeader(http.StatusOK)
	})), func() LogEvent { return e }
}

func makeNode(t *testing.T) (node *core.IpfsNode, repopath string) {
	t.Helper()

	// Create a Temporary Repo
	repoPath, err := os.MkdirTemp("", "ipfs-shell")
	if err != nil {
		t.Fatal(err)
	}

	pebbledspli := pebbleds.Plugins[0]
	pebbledspl, ok := pebbledspli.(plugin.PluginDatastore)
	if !ok {
		t.Fatal("bad datastore plugin")
	}

	err = fsrepo.AddDatastoreConfigHandler(pebbledspl.DatastoreTypeName(), pebbledspl.DatastoreConfigParser())
	if err != nil {
		t.Fatal(err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		t.Fatal(err)
	}

	// Bind ephemeral localhost ports so the test does not collide with a
	// daemon already listening on the default swarm port.
	cfg.Addresses.Swarm = []string{
		"/ip4/127.0.0.1/tcp/0",
		"/ip4/127.0.0.1/udp/0/quic-v1",
	}

	cfg.Datastore.Spec = map[string]any{
		"type":               "pebbleds",
		"prefix":             "pebble.datastore",
		"path":               "pebbleds",
		"formatMajorVersion": int(pebble.FormatNewest),
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Open the repo
	repo, err := fsrepo.Open(repoPath)
	if err != nil {
		t.Fatal(err)
	}

	// Construct the node

	nodeOptions := &core.BuildCfg{
		Online:  true,
		Routing: libp2p.NilRouterOption,
		Repo:    repo,
	}

	node, err = core.NewNode(context.Background(), nodeOptions)
	if err != nil {
		t.Fatal(err)
	}

	node.IsDaemon = true
	return
}

func TestSendTelemetry(t *testing.T) {
	if err := logging.SetLogLevel("telemetry", "DEBUG"); err != nil {
		t.Fatal(err)
	}
	// Ignore whatever the machine running the tests has set.
	t.Setenv(modeEnvVar, "")
	t.Setenv(doNotTrackEnvVar, "")
	ts, eventGetter := mockServer(t)
	defer ts.Close()

	node, repoPath := makeNode(t)

	// Create a plugin instance
	p := &telemetryPlugin{
		runOnce: true,
	}

	// Point the plugin at the mock endpoint instead of the built-in one.
	pe := &plugin.Environment{
		Repo: repoPath,
		Config: map[string]any{
			"Endpoint": ts.URL,
		},
	}
	err := p.Init(pe)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Start the plugin
	err = p.Start(node)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	e := eventGetter()
	if e.UUID != p.event.UUID {
		t.Fatal("uuid mismatch")
	}
}

// TestModeResolution covers where the mode comes from: IPFS_TELEMETRY wins over
// DO_NOT_TRACK, which wins over the config file, which wins over the default.
func TestModeResolution(t *testing.T) {
	for _, tc := range []struct {
		name       string
		telemetry  string // IPFS_TELEMETRY
		doNotTrack string // DO_NOT_TRACK
		configMode string
		want       pluginMode
	}{
		{name: "unset is on", want: modeOn},
		{name: "env off", telemetry: "off", want: modeOff},
		{name: "env auto", telemetry: "auto", want: modeAuto},
		{name: "config off", configMode: "off", want: modeOff},
		{name: "env beats config", telemetry: "on", configMode: "off", want: modeOn},
		{name: "do not track", doNotTrack: "1", want: modeOff},
		{name: "do not track true", doNotTrack: "true", want: modeOff},
		{name: "do not track beats config", doNotTrack: "1", configMode: "on", want: modeOff},
		{name: "do not track zero is not opting out", doNotTrack: "0", want: modeOn},
		{name: "env beats do not track", telemetry: "on", doNotTrack: "1", want: modeOn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(modeEnvVar, tc.telemetry)
			t.Setenv(doNotTrackEnvVar, tc.doNotTrack)

			cfg := map[string]any{}
			if tc.configMode != "" {
				cfg["Mode"] = tc.configMode
			}

			p := &telemetryPlugin{}
			if err := p.Init(&plugin.Environment{Repo: t.TempDir(), Config: cfg}); err != nil {
				t.Fatalf("Init() failed: %v", err)
			}
			if p.mode != tc.want {
				t.Fatalf("mode = %d, want %d", p.mode, tc.want)
			}
		})
	}
}

// TestEndpointFromBuild covers the built-in endpoint and the build-time knob
// that removes it, documented in the package comment.
func TestEndpointFromBuild(t *testing.T) {
	t.Setenv(modeEnvVar, "")
	t.Setenv(doNotTrackEnvVar, "")

	p := &telemetryPlugin{}
	if err := p.Init(&plugin.Environment{Repo: t.TempDir()}); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if p.endpoint != defaultEndpoint {
		t.Fatalf("endpoint = %q, want the built-in %q", p.endpoint, defaultEndpoint)
	}

	// Same as building with -ldflags "-X ...telemetry.defaultEndpoint=".
	t.Cleanup(func(orig string) func() {
		return func() { defaultEndpoint = orig }
	}(defaultEndpoint))
	defaultEndpoint = ""

	p = &telemetryPlugin{}
	if err := p.Init(&plugin.Environment{Repo: t.TempDir()}); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if p.endpoint != "" {
		t.Fatalf("endpoint = %q, want none", p.endpoint)
	}
}

// TestEndpointRetired covers the kill switch: a collector answering 410 Gone
// stops this node for good, without a Kubo release.
func TestEndpointRetired(t *testing.T) {
	t.Setenv(modeEnvVar, "")
	t.Setenv(doNotTrackEnvVar, "")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer ts.Close()

	repoPath := t.TempDir()
	env := &plugin.Environment{Repo: repoPath, Config: map[string]any{"Endpoint": ts.URL}}

	p := &telemetryPlugin{}
	if err := p.Init(env); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if err := os.WriteFile(p.uuidFilename, []byte("f81d4fae-7dec-11d0-a765-00a0c91e6bf6"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := p.sendTelemetry(); !errors.Is(err, errEndpointRetired) {
		t.Fatalf("sendTelemetry() = %v, want %v", err, errEndpointRetired)
	}
	p.retire()

	if _, err := os.Stat(p.uuidFilename); !os.IsNotExist(err) {
		t.Fatal("retiring should drop the node identifier")
	}

	// A later run stays off for that endpoint.
	next := &telemetryPlugin{}
	if err := next.Init(env); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if next.mode != modeOff {
		t.Fatalf("mode = %d, want %d after the endpoint retired", next.mode, modeOff)
	}

	// Pointing at another collector resumes reporting.
	elsewhere := &telemetryPlugin{}
	err := elsewhere.Init(&plugin.Environment{
		Repo:   repoPath,
		Config: map[string]any{"Endpoint": "https://telemetry.example.com"},
	})
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	if elsewhere.mode == modeOff {
		t.Fatal("a retired endpoint must not disable a different one")
	}
}
