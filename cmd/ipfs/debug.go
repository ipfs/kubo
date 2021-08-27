package main

import (
	"io"
	"net/http"
	"runtime"
)

func init() {
	http.HandleFunc("/debug/stack",
		func(w http.ResponseWriter, _ *http.Request) {
			_ = writeGoroutineStacks(w)
		},
	)
}

func writeGoroutineStacks(w io.Writer) error {
	buf := make([]byte, 1<<20)
	for i := 0; ; i++ {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		// if len(buf) >= 64<<20 {
		// 	// Filled 64 MB - stop there.
		// 	break
		// }
		buf = make([]byte, 2*len(buf))
	}
	_, err := w.Write(buf)
	return err
}
