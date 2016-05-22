package files

import (
	"io"
	"os"
)

// An AdvReader is like a Reader but supports getting the current file
// path and offset into the file when applicable.
type AdvReader interface {
	io.Reader
	PosInfo() *PosInfo
}

type PosInfo struct {
	Offset   uint64
	FullPath string
	Stat     os.FileInfo // can be nil
}

type advReaderAdapter struct {
	io.Reader
}

func (advReaderAdapter) PosInfo() *PosInfo {
	return nil
}

func AdvReaderAdapter(r io.Reader) AdvReader {
	switch t := r.(type) {
	case AdvReader:
		return t
	default:
		return advReaderAdapter{r}
	}
}

