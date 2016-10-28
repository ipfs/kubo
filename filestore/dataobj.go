package filestore

import (
	"fmt"
	pb "github.com/ipfs/go-ipfs/filestore/pb"
	"math"
	"time"
)

const (
	// If NoBlockData is true the Data is missing the Block data
	// as that is provided by the underlying file
	NoBlockData = 1
	// If WholeFile is true the Data object represents a complete
	// file and Size is the size of the file
	WholeFile = 2
	// If the node represents an a file but is not a leaf
	// If WholeFile is also true than it is the file's root node
	Internal = 4
	// If the block was determined to no longer be valid
	Invalid = 8
)

type DataObj struct {
	Flags uint64
	// The path to the file that holds the data for the object, an
	// empty string if there is no underlying file
	FilePath string
	Offset   uint64
	Size     uint64
	ModTime  float64
	Data     []byte
}

func (d *DataObj) NoBlockData() bool { return d.Flags&NoBlockData != 0 }
func (d *DataObj) HaveBlockData() bool { return !d.NoBlockData() }

func (d *DataObj) WholeFile() bool { return d.Flags&WholeFile != 0 }

func (d *DataObj) Internal() bool { return d.Flags&Internal != 0 }

func (d *DataObj) Invalid() bool { return d.Flags&Invalid != 0 }

func (d *DataObj) SetInvalid(val bool) {
	if val {
		d.Flags |= Invalid
	} else {
		d.Flags &^= Invalid
	}
}

func FromTime(t time.Time) float64 {
	res := float64(t.Unix())
	if res > 0 {
		res += float64(t.Nanosecond()) / 1000000000.0
	}
	return res
}

func ToTime(t float64) time.Time {
	sec, frac := math.Modf(t)
	return time.Unix(int64(sec), int64(frac*1000000000.0))
}

func (d *DataObj) StripData() DataObj {
	return DataObj{
		d.Flags, d.FilePath, d.Offset, d.Size, d.ModTime, nil,
	}
}

func (d *DataObj) KeyStr(key Key) string {
	if key.FilePath == "" {
		res := key.Format()
		res += " /"
		res += d.FilePath
		res += "//"
		res += fmt.Sprintf("%d", d.Offset)
		return res
	} else {
		return key.Format()
	}
}

func (d *DataObj) TypeStr() string {
	if d.Invalid() && d.NoBlockData() {
		return "invld"
	} else if d.NoBlockData() {
		return "leaf"
	} else if d.Internal() && d.WholeFile() {
		return "root"
	} else {
		return "other"
	}
}

func (d *DataObj) DateStr() string {
	if d.NoBlockData() {
		return ToTime(d.ModTime).Format("2006-01-02T15:04:05.000Z07:00")
	} else {
		return ""
	}
}

func (d *DataObj) Format() string {
	offset := fmt.Sprintf("%d", d.Offset)
	if d.WholeFile() {
		offset = "-"
	}
	date := ToTime(d.ModTime).Format("2006-01-02T15:04:05.000Z07:00")
	if d.Invalid() && d.NoBlockData() {
		return fmt.Sprintf("invld %s %s %d %s", d.FilePath, offset, d.Size, date)
	} else if d.NoBlockData() {
		return fmt.Sprintf("leaf  %s %s %d %s", d.FilePath, offset, d.Size, date)
	} else if d.Internal() && d.WholeFile() {
		return fmt.Sprintf("root  %s %s %d", d.FilePath, offset, d.Size)
	} else {
		return fmt.Sprintf("other %s %s %d", d.FilePath, offset, d.Size)
	}
}

func (d *DataObj) Marshal() ([]byte, error) {
	pd := new(pb.DataObj)

	pd.Flags = &d.Flags

	if d.FilePath != "" {
		pd.FilePath = &d.FilePath
	}
	if d.Offset != 0 {
		pd.Offset = &d.Offset
	}
	if d.Size != 0 {
		pd.Size_ = &d.Size
	}
	if d.Data != nil {
		pd.Data = d.Data
	}

	if d.ModTime != 0.0 {
		pd.Modtime = &d.ModTime
	}

	return pd.Marshal()
}

func (d *DataObj) Unmarshal(data []byte) error {
	pd := new(pb.DataObj)
	err := pd.Unmarshal(data)
	if err != nil {
		panic(err)
	}

	if pd.Flags != nil {
		d.Flags = *pd.Flags
	}

	if pd.FilePath != nil {
		d.FilePath = *pd.FilePath
	}
	if pd.Offset != nil {
		d.Offset = *pd.Offset
	}
	if pd.Size_ != nil {
		d.Size = *pd.Size_
	}
	if pd.Data != nil {
		d.Data = pd.Data
	}

	if pd.Modtime != nil {
		d.ModTime = *pd.Modtime
	}

	return nil
}
