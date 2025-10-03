//go:build ignore
// +build ignore

// This file contains the mock migration binary code used by tests.
// It simulates the behavior of real fs-repo-migrations binaries,
// including proper lock file handling as implemented in fs-repo-migrations.
//
// IMPORTANT: This mock MUST match the exact behavior of legacy fs-repo-15-to-16
// migration from https://github.com/ipfs/fs-repo-migrations. Do NOT modify the
// lock handling logic unless the real migration is also updated.
//
// To use this mock:
// 1. Write this source to a temp file
// 2. Compile it with: go build -o fs-repo-X-to-Y mock.go
// 3. Add the directory to PATH for tests

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type pidLockMeta struct {
	OwnerPID int
}

// isStaleLock checks if a lock file is stale (process no longer exists)
// This is the exact logic from fs-repo-migrations/tools/lock/lock.go
func isStaleLock(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var meta pidLockMeta
	if json.NewDecoder(f).Decode(&meta) != nil {
		return false
	}
	if meta.OwnerPID == 0 {
		return false
	}
	p, err := os.FindProcess(meta.OwnerPID)
	if err != nil {
		// e.g. on Windows
		return true
	}
	// On unix, os.FindProcess always is true, so we have to send
	// it a signal to see if it's alive.
	if runtime.GOOS != "windows" {
		if p.Signal(syscall.Signal(0)) != nil {
			return true
		}
	}
	return false
}

func main() {
	var path string
	var revert bool
	var verbose bool
	for _, a := range os.Args[1:] {
		if strings.HasPrefix(a, "-path=") {
			path = a[6:]
		}
		if a == "-revert" {
			revert = true
		}
		if a == "-verbose" || strings.HasPrefix(a, "-verbose=") {
			verbose = true
		}
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "missing -path=")
		os.Exit(1)
	}

	// Get from/to versions from environment variables set by test
	from := os.Getenv("MOCK_FROM_VERSION")
	to := os.Getenv("MOCK_TO_VERSION")
	if from == "" || to == "" {
		fmt.Fprintln(os.Stderr, "MOCK_FROM_VERSION and MOCK_TO_VERSION must be set")
		os.Exit(1)
	}

	if revert {
		from, to = to, from
		if verbose {
			fmt.Println("reverting migration")
		}
	} else {
		if verbose {
			fmt.Printf("applying %s-to-%s repo migration\n", from, to)
		}
	}

	// Exact lock logic from fs-repo-migrations
	lockPath := filepath.Join(path, "repo.lock")
	if verbose {
		fmt.Printf("locking repo at %q\n", path)
	}

	// Check if lock exists and handle stale locks
	fi, err := os.Stat(lockPath)
	if err == nil && fi.Size() > 0 {
		if isStaleLock(lockPath) {
			os.Remove(lockPath)
		} else {
			fmt.Fprintf(os.Stderr, "failed to acquire repo lock at %s\nIs a daemon running? please stop it before running migration\n", lockPath)
			os.Exit(1)
		}
	}

	// Create lock file exclusively
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to acquire repo lock at %s\nIs a daemon running? please stop it before running migration\n", lockPath)
		os.Exit(1)
	}

	// Write PID to lock file
	if err := json.NewEncoder(f).Encode(&pidLockMeta{OwnerPID: os.Getpid()}); err != nil {
		f.Close()
		os.Remove(lockPath)
		fmt.Fprintf(os.Stderr, "Error writing lock: %v\n", err)
		os.Exit(1)
	}

	// Close and clean up lock on exit
	defer func() {
		if verbose {
			fmt.Printf("releasing lock %s\n", lockPath)
		}
		f.Close()
		if err := os.Remove(lockPath); err != nil {
			if verbose {
				fmt.Printf("warning: failed to remove lock file: %v\n", err)
			}
		}
	}()

	// Print fake message for test
	fmt.Printf("fake applying %s-to-%s repo migration\n", from, to)

	// Update version file
	if err := os.WriteFile(filepath.Join(path, "version"), []byte(to), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Success message like real migration
	if revert {
		fmt.Printf("lowered version number to %s\n", to)
	} else {
		fmt.Printf("updated version file\n")
	}
}
