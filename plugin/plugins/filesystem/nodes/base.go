package fsnodes

import (
	"context"
	"io"
	"sync"

	"github.com/hugelgupf/p9/p9"
	"github.com/hugelgupf/p9/unimplfs"
	logging "github.com/ipfs/go-log"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

type FSNode interface {
	corepath.Path
	//RWLocker
	Stat() (p9.QID, error)
}

const ( //type
	tVirtual = iota
	tIPFS
)
const ( //device
	dMemory = iota
	dIPFS
)

const ( //FS namespaces
	nRoot = "root"
)
const ( //9P paths
	pVirtualRoot uint64 = iota
	//pIPFSRoot
	//pIpnsRoot
	pPinRoot
	//pKeyRoot
)

var _ p9.File = (*Base)(nil)

//var _ FSNode = (*Base)(nil)

//TODO: docs
// Base is a foundational node, intended to be embedded/extended
type Base struct {
	unimplfs.NoopFile
	p9.DefaultWalkGetAttr
	Qid  p9.QID
	meta p9.Attr

	Ctx    context.Context
	Logger logging.EventLogger
}

type IPFSBase struct {
	Base

	//Path corepath.Path
	Path corepath.Resolved
	core coreiface.CoreAPI
}

type ResourceRef struct {
	sync.Mutex
	//meta   p9p.Dir
	closer io.Closer
}

/* TODO: [current] master attach(with the name) {
    global staticRoot = &root{}
    reference := staticRoot

    }

    walk {
	switch names[0] {
	if "ipfs" {
	    return fs.roots[ipfs].walk(names[1:])
	}
    }
    }
}
*/

/*
func (bn *Base) Stat(ctx context.Context) (p9.QID, error) {
	var (
		qid p9.QID
		fi  os.FileInfo
		err error
	)

	// Stat the file.
	if l.file != nil {
		fi, err = l.file.Stat()
	} else {
		fi, err = os.Lstat(l.path)
	}
	if err != nil {
		log.Printf("error stating %#v: %v", l, err)
		return qid, nil, err
	}

	// Construct the QID type.
	qid.Type = p9.ModeFromOS(fi.Mode()).QIDType()

	// Save the path from the Ino.
	qid.Path = fi.Sys().(*syscall.Stat_t).Ino
	return qid, fi, nil
}
*/
