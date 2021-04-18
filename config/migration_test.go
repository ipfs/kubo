package config

import (
	"encoding/json"
	"testing"
)

func TestMigrationDecode(t *testing.T) {
	str := `
		{
			"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
			"Keep": "cache",
			"Peers": [
				{
					"ID": "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5",
					"Addrs": ["/ip4/127.0.0.1/tcp/4001", "/ip4/127.0.0.1/udp/4001/quic"]
				},
				{
					"ID": "12D3KooWGC6TvWhfajpgX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ7",
					"Addrs": ["/ip4/10.0.0.2/tcp/4001"]
				}
			]
		}
	`

	var cfg Migration
	if err := json.Unmarshal([]byte(str), &cfg); err != nil {
		t.Errorf("failed while unmarshalling migration struct: %s", err)
	}

	if len(cfg.DownloadSources) != 3 {
		t.Fatal("wrong number of DownloadSources")
	}
	expect := []string{"IPFS", "HTTP", "127.0.0.1"}
	for i := range expect {
		if cfg.DownloadSources[i] != expect[i] {
			t.Errorf("wrong DownloadSource at %d", i)
		}
	}

	if cfg.Keep != "cache" {
		t.Error("wrong value for Keep")
	}

	if len(cfg.Peers) != 2 {
		t.Fatal("wrong number of peers")
	}

	peer := cfg.Peers[0]
	if peer.ID.String() != "12D3KooWGC6TvWhfapngX6wvJHMYvKpDMXPb3ZnCZ6dMoaMtimQ5" {
		t.Errorf("wrong ID for first peer")
	}
	if len(peer.Addrs) != 2 {
		t.Error("wrong number of addrs for first peer")
	}
	if peer.Addrs[0].String() != "/ip4/127.0.0.1/tcp/4001" {
		t.Error("wrong first addr for first peer")
	}
	if peer.Addrs[1].String() != "/ip4/127.0.0.1/udp/4001/quic" {
		t.Error("wrong second addr for first peer")
	}

	peer = cfg.Peers[1]
	if len(peer.Addrs) != 1 {
		t.Fatal("wrong number of addrs for second peer")
	}
	if peer.Addrs[0].String() != "/ip4/10.0.0.2/tcp/4001" {
		t.Error("wrong first addr for second peer")
	}
}
