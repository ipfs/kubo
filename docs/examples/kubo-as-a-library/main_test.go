package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestExample(t *testing.T) {
	t.Log("Starting go run main.go...")
	start := time.Now()

	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append(os.Environ(), "GOLOG_LOG_LEVEL=error") // reduce libp2p noise

	// Stream output to both test log and capture buffer for verification
	// This ensures we see progress even if the process is killed
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)

	err := cmd.Run()

	elapsed := time.Since(start)
	t.Logf("Command completed in %v", elapsed)

	out := buf.String()
	if err != nil {
		t.Fatalf("running example (%v):\n%s", err, out)
	}

	if !strings.Contains(out, "All done!") {
		t.Errorf("example did not complete successfully, output:\n%s", out)
	}
}
