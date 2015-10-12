package args

import (
	"flag"
	"fmt"
	"os"
)

func Parse() {
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "unexpected positional arguments\n")
		os.Exit(2)
	}
}
