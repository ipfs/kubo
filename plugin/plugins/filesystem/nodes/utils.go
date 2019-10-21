package fsnodes

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"syscall"
	"time"

	"github.com/hugelgupf/p9/p9"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	unixpb "github.com/ipfs/go-unixfs/pb"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
	corepath "github.com/ipfs/interface-go-ipfs-core/path"
)

const (
	// TODO [2019.09.12; anyone]
	// Start a discussion around block sizes
	// should we use the de-facto standard of 4KiB or use our own of 256KiB?
	// context: https://github.com/ipfs/go-ipfs/pull/6612/files#r322989041
	ipfsBlockSize = 256 << 10
	saltSize      = 32

	// TODO: move these
	// Linux errno values for non-Linux systems; 9p2000.L compliance
	ENOTDIR = syscall.Errno(0x14)
	ENOENT  = syscall.ENOENT //TODO: migrate to platform independent value

	// pedantic POSIX stuff
	S_IROTH p9.FileMode = p9.Read
	S_IWOTH             = p9.Write
	S_IXOTH             = p9.Exec

	S_IRGRP = S_IROTH << 3
	S_IWGRP = S_IWOTH << 3
	S_IXGRP = S_IXOTH << 3

	S_IRUSR = S_IRGRP << 3
	S_IWUSR = S_IWGRP << 3
	S_IXUSR = S_IXGRP << 3

	S_IRWXO = S_IROTH | S_IWOTH | S_IXOTH
	S_IRWXG = S_IRGRP | S_IWGRP | S_IXGRP
	S_IRWXU = S_IRUSR | S_IWUSR | S_IXUSR

	IRWXA = S_IRWXU | S_IRWXG | S_IRWXO            // 0777
	IRXA  = IRWXA &^ (S_IWUSR | S_IWGRP | S_IWOTH) // 0555
)

var (
	errWalkOpened    = errors.New("this fid is open")
	errKeyNotInStore = errors.New("requested key was not found in the key store")

	// NOTE [2019.09.12]: QID's have a high collision probability
	// as a result we add a salt to hashes to attempt to mitigate this
	// for more context see: https://github.com/ipfs/go-ipfs/pull/6612#discussion_r321038649
	salt []byte
)

type directoryStream struct {
	entryChan <-chan coreiface.DirEntry
	cursor    uint64
	err       error
}

type WalkRef interface {
	p9.File

	/* CheckWalk should make sure that the current reference adheres to the restrictions
	of 'walk(5)'
	In particular the reference must not be open for I/O, or otherwise already closed
	*/
	CheckWalk() error

	/* Fork allocates a reference derived from itself
	The returned reference should be at the same path as the existing walkRef
	the new reference is to act like the starting point `newfid` during `Walk`
	e.g.
	`newFid` originally references the same data as the origin `walkRef`
	but is closed separately from the origin
	operations such as `Walk` will modify `newFid` without affecting the origin `walkRef`
	operations such as `Open` should prevent all references to the same path from opening
	etc. in compliance with 'walk(5)'
	*/
	Fork() (WalkRef, error)

	/* QID should check that the node's path is walkable
	by constructing the QID for its path
	*/
	QID() (p9.QID, error)

	/* Step should return a reference that is tracking the result of
	the node's current path + "name"
	implementation of this is fs specific
	it is valid to return a new reference or the same reference modified
	within or outside of your own fs boundaries
	as long as `QID` is ready to be called on the resulting node
	*/
	Step(name string) (WalkRef, error)

	/* Backtrack must handle `..` request
	returning a reference to the node behind the current node (or itself in the case of the root)
	the same comment about implementation of `Step` applies here
	*/
	Backtrack() (parentRef WalkRef, err error)
}

func walker(ref WalkRef, names []string) ([]p9.QID, p9.File, error) {
	err := ref.CheckWalk()
	if err != nil {
		return nil, nil, err
	}

	curRef, err := ref.Fork()
	if err != nil {
		return nil, nil, err
	}

	// walk(5)
	// It is legal for nwname to be zero, in which case newfid will represent the same file as fid
	//  and the walk will usually succeed
	if shouldClone(names) {
		qid, err := ref.QID()
		if err != nil {
			return nil, nil, err
		}
		return []p9.QID{qid}, curRef, nil
	}

	qids := make([]p9.QID, 0, len(names))

	for _, name := range names {
		switch name {
		default:
			// get ready to step forward; maybe across FS bounds
			curRef, err = curRef.Step(name)

		case ".":
			// don't prepare to move at all

		case "..":
			// get ready to step backwards; maybe across FS bounds
			curRef, err = curRef.Backtrack()
		}

		if err != nil {
			return qids, nil, err
		}

		// commit to the step
		qid, err := curRef.QID()
		if err != nil {
			return qids, nil, err
		}

		// set on success, we stepped forward
		qids = append(qids, qid)

	}

	return qids, curRef, nil
}

func init() {
	salt = make([]byte, saltSize)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		panic(err)
	}
}

func shouldClone(names []string) bool {
	switch len(names) {
	case 0: // empty path
		return true
	case 1: // self?
		pc := names[0]
		return pc == "." || pc == ""
	default:
		return false
	}
}

func ipldStat(ctx context.Context, attr *p9.Attr, node ipld.Node, mask p9.AttrMask) (p9.AttrMask, error) {
	var filledAttrs p9.AttrMask
	ufsNode, err := unixfs.ExtractFSNode(node)
	if err != nil {
		return filledAttrs, err
	}

	if mask.Mode {
		tBits, err := unixfsTypeTo9Type(ufsNode.Type())
		if err != nil {
			return filledAttrs, err
		}
		attr.Mode = tBits
		filledAttrs.Mode = true
	}

	if mask.Blocks {
		//TODO: when/if UFS supports this metadata field, use it instead
		attr.BlockSize, filledAttrs.Blocks = ipfsBlockSize, true
	}

	if mask.Size {
		attr.Size, filledAttrs.Size = ufsNode.FileSize(), true
	}

	//TODO [eventually]: handle time metadata in new UFS format standard

	return filledAttrs, nil
}

func cidToQPath(cid cid.Cid) uint64 {
	hasher := fnv.New64a()
	if _, err := hasher.Write(salt); err != nil {
		panic(err)
	}
	if _, err := hasher.Write(cid.Bytes()); err != nil {
		panic(err)
	}
	return hasher.Sum64()
}

//NOTE [2019.09.11]: IPFS CoreAPI abstracts over HAMT structures; Unixfs returns raw type

func coreTypeTo9Type(ct coreiface.FileType) (p9.FileMode, error) {
	switch ct {
	case coreiface.TDirectory:
		return p9.ModeDirectory, nil
	case coreiface.TSymlink:
		return p9.ModeSymlink, nil
	case coreiface.TFile:
		return p9.ModeRegular, nil
	default:
		return p9.ModeRegular, fmt.Errorf("CoreAPI data type %q was not expected, treating as regular file", ct)
	}
}

//TODO: see if we can remove the need for this; rely only on the core if we can
func unixfsTypeTo9Type(ut unixpb.Data_DataType) (p9.FileMode, error) {
	switch ut {
	//TODO: directories and hamt shards are not synonymous; HAMTs may need special handling
	case unixpb.Data_Directory, unixpb.Data_HAMTShard:
		return p9.ModeDirectory, nil
	case unixpb.Data_Symlink:
		return p9.ModeSymlink, nil
	case unixpb.Data_File:
		return p9.ModeRegular, nil
	default:
		return p9.ModeRegular, fmt.Errorf("UFS data type %q was not expected, treating as regular file", ut)
	}
}

func coreEntTo9Ent(coreEnt coreiface.DirEntry) (p9.Dirent, error) {
	entType, err := coreTypeTo9Type(coreEnt.Type)
	if err != nil {
		return p9.Dirent{}, err
	}

	return p9.Dirent{
		Name: coreEnt.Name,
		Type: entType.QIDType(),
		QID: p9.QID{
			Type: entType.QIDType(),
			Path: cidToQPath(coreEnt.Cid),
		},
	}, nil
}

func mfsTypeToNineType(nt mfs.NodeType) (entType p9.QIDType, err error) {
	switch nt {
	//mfsEnt.Type; mfs.NodeType(t) {
	case mfs.TFile:
		entType = p9.TypeRegular
	case mfs.TDir:
		entType = p9.TypeDir
	default:
		err = fmt.Errorf("unexpected node type %v", nt)
	}
	return
}

func mfsEntTo9Ent(mfsEnt mfs.NodeListing) (p9.Dirent, error) {
	pathCid, err := cid.Decode(mfsEnt.Hash)
	if err != nil {
		return p9.Dirent{}, err
	}

	t, err := mfsTypeToNineType(mfs.NodeType(mfsEnt.Type))
	if err != nil {
		return p9.Dirent{}, err
	}

	return p9.Dirent{
		Name: mfsEnt.Name,
		Type: t,
		QID: p9.QID{
			Type: t,
			Path: cidToQPath(pathCid),
		},
	}, nil
}

func timeStamp(attr *p9.Attr, mask p9.AttrMask) {
	now := time.Now()
	if mask.ATime {
		attr.ATimeSeconds, attr.ATimeNanoSeconds = uint64(now.Unix()), uint64(now.UnixNano())
	}
	if mask.MTime {
		attr.MTimeSeconds, attr.MTimeNanoSeconds = uint64(now.Unix()), uint64(now.UnixNano())
	}
	if mask.CTime {
		attr.CTimeSeconds, attr.CTimeNanoSeconds = uint64(now.Unix()), uint64(now.UnixNano())
	}
}

func offlineAPI(core coreiface.CoreAPI) coreiface.CoreAPI {
	oAPI, err := core.WithOptions(coreoptions.Api.Offline(true))
	if err != nil {
		panic(err)
	}
	return oAPI
}

func flatReaddir(ents []p9.Dirent, offset uint64, count uint32) ([]p9.Dirent, error) {
	shouldExit, err := boundCheck(offset, len(ents))
	if shouldExit {
		return nil, err
	}

	subSlice := ents[offset:]
	if len(subSlice) > int(count) {
		subSlice = subSlice[:count]
	}
	return subSlice, nil
}

// boundCheck assures operation arguments are valid
// returns true if the caller should return immediately with our values
func boundCheck(offset uint64, length int) (bool, error) {
	switch {
	case offset == uint64(length):
		return true, nil // EOS
	case offset > uint64(length):
		return true, fmt.Errorf("offset %d extends beyond directory bound %d", offset, length)
	default:
		// not at end of stream and okay to continue
		return false, nil
	}
}

func coreToQid(ctx context.Context, path corepath.Path, core coreiface.CoreAPI) (p9.QID, error) {
	var qid p9.QID
	// translate from abstract path to CoreAPI resolved path
	resolvedPath, err := core.ResolvePath(ctx, path)
	if err != nil {
		return qid, err
	}

	// inspected to derive 9P QID
	attr := new(p9.Attr)
	_, err = coreAttr(ctx, attr, resolvedPath, core, p9.AttrMask{Mode: true})
	if err != nil {
		return qid, err
	}

	qid.Type = attr.Mode.QIDType()
	qid.Path = cidToQPath(resolvedPath.Cid())
	return qid, nil
}
