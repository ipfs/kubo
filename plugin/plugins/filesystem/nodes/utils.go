package fsnodes

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"time"

	"github.com/djdv/p9/p9"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	"github.com/ipfs/go-mfs"
	"github.com/ipfs/go-unixfs"
	unixpb "github.com/ipfs/go-unixfs/pb"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	coreoptions "github.com/ipfs/interface-go-ipfs-core/options"
)

const (
	// TODO [2019.09.12; anyone]
	// Start a discussion around block sizes
	// should we use the de-facto standard of 4KiB or use our own of 256KiB?
	// context: https://github.com/ipfs/go-ipfs/pull/6612/files#r322989041
	ipfsBlockSize = 256 << 10
	saltSize      = 32
)

var errWalkOpened = errors.New("this fid is open")
var errKeyNotInStore = errors.New("requested key was not found in the key store")

// NOTE [2019.09.12]: QID's have a high collision probability
// as a result we add a salt to hashes to attempt to mitigate this
// for more context see: https://github.com/ipfs/go-ipfs/pull/6612#discussion_r321038649
var salt []byte

type directoryStream struct {
	entryChan <-chan coreiface.DirEntry
	cursor    uint64
	eos       bool // have seen end of stream?
	err       error
}

type walkRef interface {
	p9.File
	// returns the QID of the reference
	QID() p9.QID // implemented on Base

	// returns a derived instance that is ready to step
	Derive() walkRef // implemented per filesystem

	// Step should try to step to "name", resulting in a ready to use derivative
	Step(name string) (walkRef, error)

	//TODO: implement on base class
	// Backtrack returns a reference to the node's parent
	Backtrack() (parentRef walkRef, err error)
}

func walker(ref walkRef, names []string) ([]p9.QID, p9.File, error) {
	dbgL := logging.Logger("walker") //TODO: remove this or use the reference log, or something
	dbgL.Debugf("ref: (%p){%T}", ref, ref)

	curRef := ref.Derive()

	// walk(5)
	// It is legal for nwname to be zero, in which case newfid will represent the same file as fid
	//  and the walk will usually succeed
	if shouldClone(names) {
		dbgL.Debugf("cloning: %p -> %p", ref, curRef)
		return []p9.QID{curRef.QID()}, curRef, nil
	}

	qids := make([]p9.QID, 0, len(names))

	var err error
	for _, name := range names {
		switch name {
		default:
			// step forward
			dbgL.Debugf("stepping to: %q", name)
			curRef, err = curRef.Step(name)

		case ".":
			// stay
			// qid = qids[len(qids)-1]
			// continue
			dbgL.Debugf(`staying put: "."`)

		case "..":
			// step back
			dbgL.Debugf(`backtracking to: ".."`)
			curRef, err = curRef.Backtrack()
		}

		if err != nil {
			dbgL.Error(err)
			return qids, nil, err
		}

		dbgL.Debugf("currentRef: (%p){%T}", curRef, curRef)
		qids = append(qids, curRef.QID())
	}

	dbgL.Debugf("returned ref: (%p){%T}", curRef, curRef)
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

const ( // pedantic POSIX stuff
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
