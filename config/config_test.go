package config

import (
	"fmt"
	"testing"
)

func TestConfig(t *testing.T) {

	cfg, err := LoadConfig("")
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf(cfg.Datastore.Path)
}
