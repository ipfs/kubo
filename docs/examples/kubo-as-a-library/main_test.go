package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestExample(t *testing.T) {
	cmd := exec.CommandContext(t.Context(), "go", "run", "main.go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running example: %v\nOutput:\n%s", err, out)
	}
	if !strings.Contains(string(out), "All done!") {
		t.Errorf("example did not complete successfully\nOutput:\n%s", out)
	}
}
