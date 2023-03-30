package main

import (
	"os"

	"github.com/ipfs/kubo/cmd/ipfs/kubo"
)

// main roadmap:
// - parse the commandline to get a cmdInvocation
// - if user requests help, print it and exit.
// - run the command invocation
// - output the response
// - if anything fails, print error, maybe with help
func main() {
	os.Exit(mainRet())
}

func mainRet() (exitCode int) {
	return kubo.Start(kubo.BuildDefaultEnv)
}
