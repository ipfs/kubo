package filestore

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	dshelp "github.com/ipfs/go-ipfs/thirdparty/ds-help"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	base32 "gx/ipfs/Qmb1DA2A9LS2wR4FFweB4uEDomFsdmnw1VLawLE1yQzudj/base32"
)

type Key struct {
	Hash     string
	FilePath string // empty string if not given
	Offset   int64  // -1 if not given
}

func ParseKey(str string) (*DbKey, error) {
	idx := strings.Index(str, "/")
	var key *DbKey
	if idx == -1 {
		idx = len(str)
	}
	if idx != 0 { // we have a Hash
		mhash := str[:idx]
		c, err := cid.Decode(mhash)
		if err != nil {
			return nil, err
		}
		key = CidToKey(c)
	} else {
		key = &DbKey{}
	}
	if idx == len(str) { // we just have a hash
		return key, nil
	}
	str = str[idx+1:]
	parseRest(&key.Key, str)
	return key, nil
}

func ParseDsKey(str string) Key {
	idx := strings.Index(str[1:], "/") + 1
	if idx == 0 {
		return Key{str, "", -1}
	}
	key := Key{Hash: str[:idx]}
	str = str[idx+1:]
	parseRest(&key, str)
	return key
}

func parseRest(key *Key, str string) {
	filename := strings.Trim(str, "0123456789")
	if len(filename) <= 2 || filename[len(filename)-2:] != "//" || len(str) == len(filename) {
		key.FilePath = filename
		key.Offset = -1
		return
	}
	offsetStr := str[len(filename):]
	key.FilePath = filename[:len(filename)-2]
	key.Offset, _ = strconv.ParseInt(offsetStr, 10, 64)
}

func (k Key) String() string {
	str := k.Hash
	if k.FilePath == "" {
		return str
	}
	str += "/"
	str += k.FilePath
	if k.Offset == -1 {
		return str
	}
	str += "//"
	str += strconv.FormatInt(k.Offset, 10)
	return str
}

func (k Key) Bytes() []byte {
	if k.FilePath == "" {
		return []byte(k.Hash)
	}
	buf := bytes.NewBuffer(nil)
	if k.Offset == -1 {
		fmt.Fprintf(buf, "%s/%s", k.Hash, k.FilePath)
	} else {
		fmt.Fprintf(buf, "%s/%s//%d", k.Hash, k.FilePath, k.Offset)
	}
	return buf.Bytes()
}

func (k Key) Cid() (*cid.Cid, error) {
	binary, err := base32.RawStdEncoding.DecodeString(k.Hash[1:])
	if err != nil {
		return nil, err
	}
	return cid.Cast(binary)
}

type DbKey struct {
	Key
	Bytes []byte
	cid   *cid.Cid
}

func ParseDbKey(key string) *DbKey {
	return &DbKey{
		Key:   ParseDsKey(key),
		Bytes: []byte(key),
	}
}

func NewDbKey(hash string, filePath string, offset int64, cid *cid.Cid) *DbKey {
	key := &DbKey{Key: Key{hash, filePath, offset}, cid: cid}
	key.Bytes = key.Key.Bytes()
	return key
}

func HashToKey(hash string) *DbKey {
	return NewDbKey(hash, "", -1, nil)
}

func CidToKey(c *cid.Cid) *DbKey {
	return NewDbKey(dshelp.CidToDsKey(c).String(), "", -1, c)
}

func (k *DbKey) Cid() (*cid.Cid, error) {
	if k.cid == nil {
		var err error
		k.cid, err = k.Key.Cid()
		if err != nil {
			return nil, err
		}
	}
	return k.cid, nil
}

type havecid interface {
	Cid() (*cid.Cid, error)
}

func MHash(k havecid) string {
	key, err := k.Cid()
	if err != nil {
		return "??????????????????????????????????????????????"
	}
	return key.String()
}

func (k Key) Format() string {
	if k.FilePath == "" {
		return MHash(k)
	}
	return Key{MHash(k), k.FilePath, k.Offset}.String()
}

func (k *DbKey) Format() string {
	mhash := ""
	if k.Hash != "" {
		mhash = MHash(k)
	}
	if k.FilePath == "" {
		return mhash
	}
	return Key{mhash, k.FilePath, k.Offset}.String()
}
