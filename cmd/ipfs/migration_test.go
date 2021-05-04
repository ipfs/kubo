package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations/ipfsfetcher"
)

var testConfig = `
{
	"Bootstrap": [
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	],
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1", "https://127.0.1.1"],
		"Keep": "cache"
	},
	"Peering": {
		"Peers": [
			{
				"ID": "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
				"Addrs": ["/ip4/127.0.0.1/tcp/4001", "/ip4/127.0.0.1/udp/4001/quic"]
			}
		]
	}
}
`

func TestReadMigrationConfigDefaults(t *testing.T) {
	tmpDir := makeConfig("{}")
	defer os.RemoveAll(tmpDir)

	cfg, err := readMigrationConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Keep != config.DefaultMigrationKeep {
		t.Error("expected default value for Keep")
	}

	if len(cfg.DownloadSources) != len(config.DefaultMigrationDownloadSources) {
		t.Fatal("expected default number of download sources")
	}
	for i, src := range config.DefaultMigrationDownloadSources {
		if cfg.DownloadSources[i] != src {
			t.Errorf("wrong DownloadSource: %s", cfg.DownloadSources[i])
		}
	}
}

func TestReadMigrationConfigErrors(t *testing.T) {
	tmpDir := makeConfig(`{"Migration": {"Keep": "badvalue"}}`)
	defer os.RemoveAll(tmpDir)

	_, err := readMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.HasPrefix(err.Error(), "unknown") {
		t.Fatal("did not get expected error:", err)
	}

	os.RemoveAll(tmpDir)
	_, err = readMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}

	bootstrap, peers := readIpfsConfig(&tmpDir)
	if bootstrap != nil {
		t.Error("expected nil bootstrap")
	}
	if peers != nil {
		t.Error("expected nil peers")
	}

	tmpDir = makeConfig(`}{`)
	defer os.RemoveAll(tmpDir)
	_, err = readMigrationConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadMigrationConfig(t *testing.T) {
	tmpDir := makeConfig(testConfig)
	defer os.RemoveAll(tmpDir)

	cfg, err := readMigrationConfig(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.DownloadSources) != 4 {
		t.Fatal("wrong number of DownloadSources")
	}
	expect := []string{"IPFS", "HTTP", "127.0.0.1", "https://127.0.1.1"}
	for i := range expect {
		if cfg.DownloadSources[i] != expect[i] {
			t.Errorf("wrong DownloadSource at %d", i)
		}
	}

	if cfg.Keep != "cache" {
		t.Error("wrong value for Keep")
	}
}

func TestReadIpfsConfig(t *testing.T) {
	tmpDir := makeConfig(testConfig)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers := readIpfsConfig(nil)
	if bootstrap != nil || peers != nil {
		t.Fatal("expected nil ipfs config items")
	}

	bootstrap, peers = readIpfsConfig(&tmpDir)
	if len(bootstrap) != 2 {
		t.Fatal("wrong number of bootstrap addresses")
	}
	if bootstrap[0] != "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt" {
		t.Fatal("wrong bootstrap address")
	}

	if len(peers) != 1 {
		t.Fatal("wrong number of peers")
	}

	peer := peers[0]
	if peer.ID.String() != "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5" {
		t.Errorf("wrong ID for first peer")
	}
	if len(peer.Addrs) != 2 {
		t.Error("wrong number of addrs for first peer")
	}
}

func TestReadPartialIpfsConfig(t *testing.T) {
	const (
		configBadBootstrap = `
{
	"Bootstrap": "unreadable",
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
		"Keep": "cache"
	},
	"Peering": {
		"Peers": [
			{
				"ID": "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
				"Addrs": ["/ip4/127.0.0.1/tcp/4001", "/ip4/127.0.0.1/udp/4001/quic"]
			}
		]
	}
}
`
		configBadPeers = `
{
	"Bootstrap": [
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ"
	],
	"Migration": {
		"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
		"Keep": "cache"
	},
	"Peering": "Unreadable-data"
}
`
	)

	tmpDir := makeConfig(configBadBootstrap)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers := readIpfsConfig(&tmpDir)
	if bootstrap != nil {
		t.Fatal("expected nil bootstrap")
	}
	if len(peers) != 1 {
		t.Fatal("wrong number of peers")
	}
	if len(peers[0].Addrs) != 2 {
		t.Error("wrong number of addrs for first peer")
	}
	os.RemoveAll(tmpDir)

	tmpDir = makeConfig(configBadPeers)
	defer os.RemoveAll(tmpDir)

	bootstrap, peers = readIpfsConfig(&tmpDir)
	if peers != nil {
		t.Fatal("expected nil peers")
	}
	if len(bootstrap) != 2 {
		t.Fatal("wrong number of bootstrap addresses")
	}
	if bootstrap[0] != "/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt" {
		t.Fatal("wrong bootstrap address")
	}
}

func makeConfig(configData string) string {
	tmpDir, err := ioutil.TempDir("", "migration_test")
	if err != nil {
		panic(err)
	}

	cfgFile, err := os.Create(filepath.Join(tmpDir, "config"))
	if err != nil {
		panic(err)
	}
	if _, err = cfgFile.Write([]byte(configData)); err != nil {
		panic(err)
	}
	if err = cfgFile.Close(); err != nil {
		panic(err)
	}
	return tmpDir
}

func TestGetMigrationFetcher(t *testing.T) {
	var f migrations.Fetcher
	var err error

	cfg := &config.Migration{}

	cfg.DownloadSources = []string{"ftp://bad.gateway.io"}
	_, err = getMigrationFetcher(cfg, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "bad gateway addr") {
		t.Fatal("Expected bad gateway address error, got:", err)
	}

	cfg.DownloadSources = []string{"::bad.gateway.io"}
	_, err = getMigrationFetcher(cfg, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "bad gateway addr") {
		t.Fatal("Expected bad gateway address error, got:", err)
	}

	cfg.DownloadSources = []string{"http://localhost"}
	f, err = getMigrationFetcher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*migrations.HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	cfg.DownloadSources = []string{"ipfs"}
	f, err = getMigrationFetcher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*ipfsfetcher.IpfsFetcher); !ok {
		t.Fatal("expected IpfsFetcher")
	}

	cfg.DownloadSources = []string{"http"}
	f, err = getMigrationFetcher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.(*migrations.HttpFetcher); !ok {
		t.Fatal("expected HttpFetcher")
	}

	cfg.DownloadSources = []string{"IPFS", "HTTPS"}
	f, err = getMigrationFetcher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok := f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 2 {
		t.Fatal("expected 2 fetchers in MultiFetcher")
	}

	cfg.DownloadSources = []string{"ipfs", "https", "some.domain.io"}
	f, err = getMigrationFetcher(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	mf, ok = f.(*migrations.MultiFetcher)
	if !ok {
		t.Fatal("expected MultiFetcher")
	}
	if mf.Len() != 3 {
		t.Fatal("expected 3 fetchers in MultiFetcher")
	}

	cfg.DownloadSources = nil
	_, err = getMigrationFetcher(cfg, nil)
	if err == nil {
		t.Fatal("expected error when no sources specified")
	}

	cfg.DownloadSources = []string{"", ""}
	_, err = getMigrationFetcher(cfg, nil)
	if err == nil {
		t.Fatal("expected error when empty string fetchers specified")
	}
}
