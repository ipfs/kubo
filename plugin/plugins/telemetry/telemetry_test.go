package telemetry

import (
	"context"
	"encoding/json"
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

	cfg.Datastore.Spec = map[string]interface{}{
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
	ts, eventGetter := mockServer(t)
	defer ts.Close()

	node, repoPath := makeNode(t)

	// Create a plugin instance
	p := &telemetryPlugin{
		runOnce: true,
	}

	// Initialize the plugin
	pe := &plugin.Environment{
		Repo:   repoPath,
		Config: nil,
	}
	err := p.Init(pe)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	p.endpoint = ts.URL

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
