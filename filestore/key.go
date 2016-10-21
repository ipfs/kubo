package filestore

import (
	"strings"
	"strconv"
)

type Key struct {
	Hash     string
	FilePath string // empty string if not given
	Offset   int64  // -1 if not given
}

func (k Key) String() string {
	str := k.Hash
	if k.FilePath == "" {return str}
	str += "//"
	str += k.FilePath
	if k.Offset == -1 {return str}
	str += "//"
	str += strconv.FormatInt(k.Offset, 10)
	return str
}

func ParseKey(key string) Key {
	idx := strings.Index(key, "//")
	if (idx == -1) {
		return Key{key,"",-1}
	}
	hash := key[:idx]
	key = key[idx+2:]
	filename := strings.Trim(key, "0123456789")
	if len(filename) <= 2 || filename[len(filename)-2:] != "//" || len(key) == len(filename){
		return Key{hash,filename,-1}
	}
	offsetStr := key[len(filename):]
	filename = filename[:len(filename)-2]
	offset,_ := strconv.ParseInt(offsetStr, 10, 64)
	return Key{hash,filename,offset}
}
