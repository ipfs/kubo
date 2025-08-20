// Package cli provides testing utilities for IPFS CLI commands.
package cli

func MustVal[V any](val V, err error) V {
	if err != nil {
		panic(err)
	}
	return val
}
