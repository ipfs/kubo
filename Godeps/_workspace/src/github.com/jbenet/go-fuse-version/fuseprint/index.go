package main

import (
	"fmt"
	"os"

	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
)

func main() {
	sys, err := fuseversion.LocalFuseSystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	fmt.Printf("FuseVersion, AgentVersion, Agent\n")
	for _, s := range *sys {
		fmt.Printf("%s, %s, %s\n", s.FuseVersion, s.AgentVersion, s.AgentName)
	}
}
