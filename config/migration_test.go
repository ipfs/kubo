package config

import (
	"encoding/json"
	"testing"
)

func TestMigrationDecode(t *testing.T) {
	str := `
		{
			"DownloadSources": ["IPFS", "HTTP", "127.0.0.1"],
			"Keep": "cache"
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
}
