// Command gen-rpc-client generates typed Go client methods for the Kubo HTTP
// RPC API by walking the command tree defined in core/commands.
//
// Usage:
//
//	go run ./tools/gen-rpc-client -output ./client/rpc/
//	go run ./tools/gen-rpc-client -check  ./client/rpc/
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	corecmds "github.com/ipfs/kubo/core/commands"
)

func main() {
	outputDir := flag.String("output", "", "directory to write generated files to")
	checkDir := flag.String("check", "", "directory to compare generated files against (exits 1 if stale)")
	flag.Parse()

	if *outputDir == "" && *checkDir == "" {
		fmt.Fprintln(os.Stderr, "usage: gen-rpc-client -output <dir>  or  gen-rpc-client -check <dir>")
		os.Exit(2)
	}

	commands := walkCommandTree(corecmds.Root, "")
	files, err := generateFiles(commands)
	if err != nil {
		fmt.Fprintf(os.Stderr, "generation failed: %v\n", err)
		os.Exit(1)
	}

	// generate smoke test alongside client files
	smokeContent, err := generateSmokeTest(commands)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke test generation failed: %v\n", err)
		os.Exit(1)
	}
	files["gen_smoke_test.go"] = smokeContent

	if *checkDir != "" {
		ok, err := checkGenerated(files, *checkDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "check failed: %v\n", err)
			os.Exit(1)
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "generated RPC client is out of date, run: make rpc_client")
			os.Exit(1)
		}
		fmt.Println("generated RPC client is up to date")
		return
	}

	if err := writeFiles(files, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "write failed: %v\n", err)
		os.Exit(1)
	}
	for name := range files {
		fmt.Println("wrote", filepath.Join(*outputDir, name))
	}
}

// checkGenerated compares generated content against existing files on disk.
func checkGenerated(files map[string][]byte, dir string) (bool, error) {
	ok := true
	for name, want := range files {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "  missing: %s\n", name)
				ok = false
				continue
			}
			return false, err
		}
		if string(got) != string(want) {
			fmt.Fprintf(os.Stderr, "  stale:   %s\n", name)
			ok = false
		}
	}

	// check for gen_ files on disk that we no longer generate
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "gen_") && strings.HasSuffix(e.Name(), ".go") {
			if _, exists := files[e.Name()]; !exists {
				fmt.Fprintf(os.Stderr, "  orphan:  %s\n", e.Name())
				ok = false
			}
		}
	}
	return ok, nil
}

// writeFiles writes generated files to disk.
func writeFiles(files map[string][]byte, dir string) error {
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}
