// +build test_helpers

package main

import (
	"fmt"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("%s: 1 argument required\n", os.Args[0])
		os.Exit(2)
	}
	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Printf("%s: %v\n", os.Args[0], err)
		os.Exit(2)
	}
	for i := 1; ; i++ {
		switch {
		case i*i == n:
			os.Exit(0)
		case i*i > n:
			os.Exit(1)
		}

	}
}
