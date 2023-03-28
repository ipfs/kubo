package testutils

import (
	"os"
	"runtime"
	"testing"
)

func RequiresDocker(t *testing.T) {
	if os.Getenv("TEST_NO_DOCKER") == "1" {
		t.SkipNow()
	}
}

func RequiresFUSE(t *testing.T) {
	if os.Getenv("TEST_NO_FUSE") == "1" {
		t.SkipNow()
	}
}

func RequiresExpensive(t *testing.T) {
	if os.Getenv("TEST_EXPENSIVE") == "1" || testing.Short() {
		t.SkipNow()
	}
}

func RequiresPlugins(t *testing.T) {
	if os.Getenv("TEST_NO_PLUGIN") == "1" {
		t.SkipNow()
	}
}

func RequiresLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
}
