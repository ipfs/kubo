package fusemount

import (
	"strings"
	"sync"

	"github.com/billziss-gh/cgofuse/fuse"
)

type recordBase struct {
	sync.RWMutex
	path string

	metadata fuse.Stat_t
	handles  *[]uint64
}

type mutableBase struct {
	recordBase
}

type ipfsNode struct {
	recordBase
	//fd       coreiface.UnixfsFile
	//initOnce sync.Once
	//fStat    *fuse.Stat_t
}

type ipnsNode struct {
	mutableBase
}

type ipnsKey struct {
	ipnsNode
}

type mfsNode struct {
	mutableBase
	//fd mfs.FileDescriptor
}

func (rb *recordBase) String() string {
	return rb.path
}

func (rb *recordBase) Handles() *[]uint64 {
	return rb.handles
}

func (rb *recordBase) Stat() *fuse.Stat_t {
	return &rb.metadata
}

//TODO: make a note somewhere that generic functions assume valid structs; define what "valid" means
func (rb *recordBase) Namespace() string {
	i := strings.IndexRune(rb.path[1:], '/')
	if i == -1 {
		return "root"
	}
	return rb.path[1:i]
}

func (mn *mfsNode) Namespace() string {
	return filesNamespace
}

func (rb *recordBase) Mutable() bool {
	return false
}

func (mb *mutableBase) Mutable() bool {
	return true
}
