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

// Trimmed returns the bytes as a string, with leading and trailing whitespace removed.
func (b *Buffer) Trimmed() string {
	b.m.Lock()
	defer b.m.Unlock()
	return strings.TrimSpace(b.b.String())
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
