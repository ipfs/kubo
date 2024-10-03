//go:build testrunmain
// +build testrunmain

package main_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/ipfs/kubo/cmd/ipfs/kubo"
)

// this abuses go so much that I felt dirty writing this code
// but it is the only way to do it without writing custom compiler that would
// be a clone of go-build with go-test.
func TestRunMain(t *testing.T) {
	args := flag.Args()
	os.Args = append([]string{os.Args[0]}, args...)

	ret := kubo.Start(kubo.BuildDefaultEnv)

	p := os.Getenv("IPFS_COVER_RET_FILE")
	if len(p) != 0 {
		os.WriteFile(p, []byte(fmt.Sprintf("%d\n", ret)), 0o777)
	}

	// close outputs so go testing doesn't print anything
	null, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0755)
	os.Stderr = null
	os.Stdout = null
}
