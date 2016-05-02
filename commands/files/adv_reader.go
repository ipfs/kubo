package files

import (
	"errors"
	"io"
	"time"
)

// An AdvReader is like a Reader but supports getting the current file
// path and offset into the file when applicable.
type AdvReader interface {
	io.Reader
	ExtraInfo() ExtraInfo
	SetExtraInfo(inf ExtraInfo) error
}

type ExtraInfo interface {
	Offset() int64
	AbsPath() string
	// Clone creates a copy with different offset
	Clone(offset int64) ExtraInfo
}

type PosInfo struct {
	offset  int64
	absPath string
}

func (i PosInfo) Offset() int64 { return i.offset }

func (i PosInfo) AbsPath() string { return i.absPath }

func (i PosInfo) Clone(offset int64) ExtraInfo { return PosInfo{offset, i.absPath} }

func NewPosInfo(offset int64, absPath string) PosInfo {
	return PosInfo{offset, absPath}
}

type advReaderAdapter struct {
	io.Reader
}

func (advReaderAdapter) ExtraInfo() ExtraInfo {
	return nil
}

func (advReaderAdapter) SetExtraInfo(_ ExtraInfo) error {
	return errors.New("Reader does not support setting ExtraInfo.")
}

func AdvReaderAdapter(r io.Reader) AdvReader {
	switch t := r.(type) {
	case AdvReader:
		return t
	default:
		return advReaderAdapter{r}
	}
}

type InfoForFilestore struct {
	ExtraInfo
	AddOpts interface{}
	ModTime time.Time
}

func (i InfoForFilestore) Clone(offset int64) ExtraInfo {
	return InfoForFilestore{i.ExtraInfo.Clone(offset), i.AddOpts, i.ModTime}
}
