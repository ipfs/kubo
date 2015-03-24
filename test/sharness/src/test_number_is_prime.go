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
	for i := 2; i <= n/2; i++ {
		if n%i == 0 {
			os.Exit(1)
		}
	}
	os.Exit(0)
}
