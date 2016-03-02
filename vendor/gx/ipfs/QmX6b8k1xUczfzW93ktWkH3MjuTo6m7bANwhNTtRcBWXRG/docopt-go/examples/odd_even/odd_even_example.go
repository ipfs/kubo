package main

import (
	"fmt"
	"gx/ipfs/QmX6b8k1xUczfzW93ktWkH3MjuTo6m7bANwhNTtRcBWXRG/docopt-go"
)

func main() {
	usage := `Usage: odd_even_example [-h | --help] (ODD EVEN)...

Example, try:
  odd_even_example 1 2 3 4

Options:
  -h, --help`

	arguments, _ := docopt.Parse(usage, nil, true, "", false)
	fmt.Println(arguments)
}
