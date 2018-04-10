package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"gx/ipfs/QmcpY7S3jQ47NhvLUQvSySpH5yPpYPpM6cfs9oGK6GkmW5/tools/benchmark/parse"
)

type Result struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Unit     string  `json:"unit"`
	DblValue float64 `json:"dblValue"`
}

type Parameter struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Unit  string      `json:"unit"`
	Value interface{} `json:"value"`
}

type Test struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Parameters []Parameter `json:"parameters,omitempty"`
	Results    []Result    `json:"results,omitempty"`
}

type Group struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	Tests []Test `json:"tests"`
}

type Out struct {
	Groups []Group `json:"groups"`
}

func main() {
	if err := mainErr(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func mainErr() error {
	if len(os.Args) != 2 {
		return errors.New("usage: gobench-to-json [outfile]")
	}

	benches, err := parse.ParseSet(os.Stdin)
	if err != nil {
		return err
	}

	tests := make([]Test, 0, len(benches))
	for _, b := range benches {
		for _, bench := range b {
			results := make([]Result, 0, 4)

			if bench.Measured&parse.NsPerOp != 0 {
				results = append(results, Result{
					Name:        "time/op",
					Description: "time per operation",
					Unit:        "ns",
					DblValue:    bench.NsPerOp,
				})
			}

			if bench.Measured&parse.MBPerS != 0 {
				results = append(results, Result{
					Name:        "throughput",
					Description: "throughput",
					Unit:        "MB/s",
					DblValue:    bench.MBPerS,
				})
			}

			if bench.Measured&parse.AllocsPerOp != 0 {
				results = append(results, Result{
					Name:        "allocs/op",
					Description: "number of allocations per operation",
					Unit:        "",
					DblValue:    float64(bench.AllocsPerOp), //TODO: figure out a better way
				})
			}

			if bench.Measured&parse.AllocedBytesPerOp != 0 {
				results = append(results, Result{
					Name:        "alloc B/op",
					Description: "bytes allocated per operation",
					Unit:        "B",
					DblValue:    float64(bench.AllocedBytesPerOp), //TODO: figure out a better way
				})
			}

			tests = append(tests, Test{
				Name: bench.Name,

				Results: results,
			})
		}
	}

	out := Out{
		Groups: []Group{
			{
				Name:        "go",
				Description: "gobench",

				Tests: tests,
			},
		},
	}

	b, err := json.Marshal(&out)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(os.Args[1], b, 0664)
}
