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

type DataObj struct {
	// If NoBlockData is true the Data is missing the Block data
	// as that is provided by the underlying file
	NoBlockData bool
	// If WholeFile is true the Data object represents a complete
	// file and Size is the size of the file
	WholeFile bool
	// If the node represents the file root, implies WholeFile
	FileRoot bool
	// The path to the file that holds the data for the object, an
	// empty string if there is no underlying file
	FilePath string
	Offset   uint64
	Size     uint64
	Data     []byte
}

func (d *DataObj) StripData() DataObj {
	return DataObj{
		d.NoBlockData, d.WholeFile, d.FileRoot,
		d.FilePath, d.Offset, d.Size, nil,
	}
}

func (d *DataObj) Format() string {
	offset := fmt.Sprintf("%d", d.Offset)
	if d.WholeFile {
		offset = "-"
	}
	if d.NoBlockData {
		return fmt.Sprintf("leaf  %s %s %d", d.FilePath, offset, d.Size)
	} else if d.FileRoot {
		return fmt.Sprintf("root  %s %s %d", d.FilePath, offset, d.Size)
	} else {
		return fmt.Sprintf("other %s %s %d", d.FilePath, offset, d.Size)
	}
}

func (d *DataObj) Marshal() ([]byte, error) {
	pd := new(pb.DataObj)

	if d.NoBlockData {
		pd.NoBlockData = &d.NoBlockData
	}
	if d.WholeFile {
		pd.WholeFile = &d.WholeFile
	}
	if d.FileRoot {
		pd.FileRoot = &d.FileRoot
		pd.WholeFile = nil
	}
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

	return pd.Marshal()
}

func (d *DataObj) Unmarshal(data []byte) error {
	pd := new(pb.DataObj)
	err := pd.Unmarshal(data)
	if err != nil {
		panic(err)
	}

	if pd.NoBlockData != nil {
		d.NoBlockData = *pd.NoBlockData
	}
	if pd.WholeFile != nil {
		d.WholeFile = *pd.WholeFile
	}
	if pd.FileRoot != nil {
		d.FileRoot = *pd.FileRoot
		if d.FileRoot {
			d.WholeFile = true
		}
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

	return nil
}
