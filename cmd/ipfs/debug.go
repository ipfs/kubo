package main

import (
	"net/http"

	"github.com/ipfs/go-ipfs/core/commands"
)

func init() {
	http.HandleFunc("/debug/stack",
		func(w http.ResponseWriter, _ *http.Request) {
			_ = commands.WriteAllGoroutineStacks(w)
		},
	)
}
