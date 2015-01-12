// +build windows

package tail

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/ActiveState/tail/winfile"
	"os"
)

func OpenFile(name string) (file *os.File, err error) {
	return winfile.OpenFile(name, os.O_RDONLY, 0)
}
