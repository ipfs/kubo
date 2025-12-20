package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestExample(t *testing.T) {
	out, err := exec.Command("go", "run", "main.go").Output()
	if err != nil {
		var stderr string
		if xe, ok := err.(*exec.ExitError); ok {
			stderr = string(xe.Stderr)
		}
		t.Fatalf("running example (%v): %s\n%s", err, string(out), stderr)
	}
	if !strings.Contains(string(out), "All done!") {
		t.Errorf("example did not run successfully")
	}
}
