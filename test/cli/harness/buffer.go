package harness

import (
	"strings"
	"sync"

	"github.com/ipfs/kubo/test/cli/testutils"
)

// Buffer is a thread-safe byte buffer.
type Buffer struct {
	b strings.Builder
	m sync.Mutex
}

func (b *Buffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *Buffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

// Trimmed returns the bytes as a string, but with the trailing newline removed.
// This only removes a single trailing newline, not all whitespace.
func (b *Buffer) Trimmed() string {
	b.m.Lock()
	defer b.m.Unlock()
	s := b.b.String()
	if len(s) == 0 {
		return s
	}
	if s[len(s)-1] == '\n' {
		return s[:len(s)-1]
	}
	return s
}

func (b *Buffer) Bytes() []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return []byte(b.b.String())
}

func (b *Buffer) Lines() []string {
	b.m.Lock()
	defer b.m.Unlock()
	return testutils.SplitLines(b.b.String())
}
