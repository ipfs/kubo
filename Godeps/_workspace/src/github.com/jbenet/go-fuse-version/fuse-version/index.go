package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"

	fuseversion "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-fuse-version"
)

// flags
var (
	flagSystem string
	flagOnly   string
	flagQuiet  bool
)

var usage = `usage: %s [flags]
print fuse and fuse agent versions
`

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}

	flag.StringVar(&flagSystem, "s", "", "show only one system (e.g. OSXFUSE)")
	flag.StringVar(&flagOnly, "only", "", "show one of {fuse, agent, agent-name}")
	flag.BoolVar(&flagQuiet, "q", false, "quiet output, no newline (use with --only)")
}

func main() {
	flag.Parse()
	sys, err := fuseversion.LocalFuseSystems()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	all := flagSystem == ""

	// if user specified a system and we dont have it, error out.
	if !all {
		checkExists(sys, flagSystem)
	}

	var buf bytes.Buffer
	for name, s := range sys {
		if !all && flagSystem != name {
			continue
		}

		switch flagOnly {
		case "fuse":
			fmt.Fprintf(&buf, PartString(name, "FuseVersion", s.FuseVersion))
		case "agent":
			fmt.Fprintf(&buf, PartString(name, "AgentVersion", s.AgentVersion))
		case "agent-name":
			fmt.Fprintf(&buf, PartString(name, "AgentName", s.AgentName))
		default:
			fmt.Fprintf(&buf, SystemString(name, s))
		}

		if all && flagQuiet { // if all & quiet, need to break between systems
			fmt.Fprintf(&buf, "\n")
		}
	}

	out := buf.Bytes()
	if flagQuiet {
		out = bytes.TrimSpace(out)
	}
	os.Stdout.Write(out)
}

func checkExists(all fuseversion.Systems, name string) {

	if _, found := all[name]; found {
		return
	}

	if !flagQuiet {
		fmt.Fprintf(os.Stderr, "error: %s system not found.\nHave: ")
		for name := range all {
			fmt.Fprintf(os.Stderr, "%s ", name)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
	os.Exit(1)
}

func SystemString(name string, sys fuseversion.FuseSystem) (s string) {
	s += PartString(name, "FuseVersion", sys.FuseVersion)
	s += PartString(name, "AgentVersion", sys.AgentVersion)
	s += PartString(name, "AgentName", sys.AgentName)
	return s
}

func PartString(sysname, partname, part string) string {
	if !flagQuiet {
		return fmt.Sprintf("%s.%s: %s\n", sysname, partname, part)
	}
	return part + "\t"
}
