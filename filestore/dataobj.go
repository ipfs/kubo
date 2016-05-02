package filestore

import (
	"fmt"
	pb "github.com/ipfs/go-ipfs/filestore/pb"
)

// A hack to get around the fact that the Datastore interface does not
// accept options
type DataWOpts struct {
	DataObj interface{}
	AddOpts interface{}
}

// Constants to indicate how the data should be added.
const (
	AddNoCopy = 1
	AddLink   = 2
)

const (
	// If NoBlockData is true the Data is missing the Block data
	// as that is provided by the underlying file
	NoBlockData = 1
	// If WholeFile is true the Data object represents a complete
	// file and Size is the size of the file
	WholeFile = 2
	// If the node represents the file root, implies WholeFile
	FileRoot = 4
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
	Modtime  float64
	Data     []byte
}

func (d *DataObj) NoBlockData() bool { return d.Flags&NoBlockData != 0 }

func (d *DataObj) WholeFile() bool { return d.Flags&WholeFile != 0 }

func (d *DataObj) FileRoot() bool { return d.Flags&FileRoot != 0 }

func (d *DataObj) Ivalid() bool { return d.Flags&Invalid != 0 }


func (d *DataObj) StripData() DataObj {
	return DataObj{
		d.Flags, d.FilePath, d.Offset, d.Size, d.Modtime, nil,
	}
}

func (d *DataObj) Format() string {
	offset := fmt.Sprintf("%d", d.Offset)
	if d.WholeFile() {
		offset = "-"
	}
	if d.NoBlockData() {
		return fmt.Sprintf("leaf  %s %s %d", d.FilePath, offset, d.Size)
	} else if d.FileRoot() {
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

	if d.Modtime != 0.0 {
		pd.Modtime = &d.Modtime
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

	if pd.NoBlockData != nil && *pd.NoBlockData {
		d.Flags |= NoBlockData
	}
	if pd.WholeFile != nil && *pd.WholeFile {
		d.Flags |= WholeFile
	}
	if pd.FileRoot != nil && *pd.FileRoot {
		d.Flags |= FileRoot
		d.Flags |= WholeFile
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
		d.Modtime = *pd.Modtime
	}

	return nil
}
