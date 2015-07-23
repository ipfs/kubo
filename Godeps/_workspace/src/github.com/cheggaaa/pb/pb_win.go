// +build windows

package pb

import (
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/olekukonko/ts"
)

func bold(str string) string {
	return str
}

func terminalWidth() (int, error) {
	size, err := ts.GetSize()
	return size.Col(), err
}
