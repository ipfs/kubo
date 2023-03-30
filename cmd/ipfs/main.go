package main

import (
	"os"

	"github.com/ipfs/kubo/cmd/ipfs/kubo"
)

func main() {
	os.Exit(kubo.Start(kubo.BuildDefaultEnv))
}
