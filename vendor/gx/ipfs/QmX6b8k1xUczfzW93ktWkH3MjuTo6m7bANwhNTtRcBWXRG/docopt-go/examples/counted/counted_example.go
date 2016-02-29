package main

import (
	"fmt"
	"gx/ipfs/QmX6b8k1xUczfzW93ktWkH3MjuTo6m7bANwhNTtRcBWXRG/docopt-go"
)

func main() {
	usage := `Usage: counted_example --help
       counted_example -v...
       counted_example go [go]
       counted_example (--path=<path>)...
       counted_example <file> <file>

Try: counted_example -vvvvvvvvvv
     counted_example go go
     counted_example --path ./here --path ./there
     counted_example this.txt that.txt`

	arguments, _ := docopt.Parse(usage, nil, true, "", false)
	fmt.Println(arguments)
}
