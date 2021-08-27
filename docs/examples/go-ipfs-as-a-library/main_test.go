package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestExample(t *testing.T) {
	out, err := exec.Command("go", "run", "main.go").Output()
	if err != nil {
		t.Fatalf("running example (%v)", err)
	}
	if !strings.Contains(string(out), "All done!") {
		t.Errorf("example did not run successfully")
	}
}
