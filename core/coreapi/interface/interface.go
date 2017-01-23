package iface

import (
	"context"
	"errors"
	"io"

	ipld "gx/ipfs/QmRSU5EqqWVZSNdbU51yXmVoF1uNw3JgTNB6RaiL7DZM16/go-ipld-node"
	cid "gx/ipfs/QmcTcsTvfaeEBRFo1TkFgT8sRmgi1n1LTZpecfVP8fzpGD/go-cid"
)

// type CoreAPI interface {
// 	ID() CoreID
// 	Version() CoreVersion
// }

type Link ipld.Link

type Reader interface {
	io.ReadSeeker
	io.Closer
}

type UnixfsAPI interface {
	Add(context.Context, io.Reader) (*cid.Cid, error)
	Cat(context.Context, string) (Reader, error)
	Ls(context.Context, string) ([]*Link, error)
}

// type ObjectAPI interface {
// 	New() (cid.Cid, Object)
// 	Get(string) (Object, error)
// 	Links(string) ([]*Link, error)
// 	Data(string) (Reader, error)
// 	Stat(string) (ObjectStat, error)
// 	Put(Object) (cid.Cid, error)
// 	SetData(string, Reader) (cid.Cid, error)
// 	AppendData(string, Data) (cid.Cid, error)
// 	AddLink(string, string, string) (cid.Cid, error)
// 	RmLink(string, string) (cid.Cid, error)
// }

// type ObjectStat struct {
// 	Cid            cid.Cid
// 	NumLinks       int
// 	BlockSize      int
// 	LinksSize      int
// 	DataSize       int
// 	CumulativeSize int
// }

var ErrIsDir = errors.New("object is a directory")
var ErrIsNonDag = errors.New("not a merkledag object")
var ErrOffline = errors.New("can't resolve, ipfs node is offline")
