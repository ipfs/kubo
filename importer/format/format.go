// Package format implements a data format for files in the ipfs filesystem
// It is not the only format in ipfs, but it is the one that the filesystem assumes
package format

import (
	"errors"

	"code.google.com/p/goprotobuf/proto"
)

func FilePBData(data []byte, totalsize uint64) []byte {
	pbfile := new(PBData)
	typ := PBData_File
	pbfile.Type = &typ
	pbfile.Data = data
	pbfile.Filesize = proto.Uint64(totalsize)

	data, err := proto.Marshal(pbfile)
	if err != nil {
		//this really shouldnt happen, i promise
		panic(err)
	}
	return data
}

func FolderPBData() []byte {
	pbfile := new(PBData)
	typ := PBData_Directory
	pbfile.Type = &typ

	data, err := proto.Marshal(pbfile)
	if err != nil {
		//this really shouldnt happen, i promise
		panic(err)
	}
	return data
}

func WrapData(b []byte) []byte {
	pbdata := new(PBData)
	typ := PBData_Raw
	pbdata.Data = b
	pbdata.Type = &typ

	out, err := proto.Marshal(pbdata)
	if err != nil {
		// This shouldnt happen. seriously.
		panic(err)
	}

	return out
}

func DataSize(data []byte) (uint64, error) {
	pbdata := new(PBData)
	err := proto.Unmarshal(data, pbdata)
	if err != nil {
		return 0, err
	}

	switch pbdata.GetType() {
	case PBData_Directory:
		return 0, errors.New("Cant get data size of directory!")
	case PBData_File:
		return pbdata.GetFilesize(), nil
	case PBData_Raw:
		return uint64(len(pbdata.GetData())), nil
	default:
		return 0, errors.New("Unrecognized node data type!")
	}
}
