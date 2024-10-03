package profile

import (
	"io"
	"runtime"
)

// WriteAllGoroutineStacks writes a stack trace to the given writer.
// This is distinct from the Go-provided method because it does not truncate after 64 MB.
func WriteAllGoroutineStacks(w io.Writer) error {
	// this is based on pprof.writeGoroutineStacks, and removes the 64 MB limit
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
