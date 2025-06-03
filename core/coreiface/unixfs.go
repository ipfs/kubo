package iface

import (
	"context"
	"iter"
	"os"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/core/coreiface/options"
)

type AddEvent struct {
	Name       string
	Path       path.ImmutablePath `json:",omitempty"`
	Bytes      int64              `json:",omitempty"`
	Size       string             `json:",omitempty"`
	Mode       os.FileMode        `json:",omitempty"`
	Mtime      int64              `json:",omitempty"`
	MtimeNsecs int                `json:",omitempty"`
}

// FileType is an enum of possible UnixFS file types.
type FileType int32

const (
	// TUnknown means the file type isn't known (e.g., it hasn't been
	// resolved).
	TUnknown FileType = iota
	// TFile is a regular file.
	TFile
	// TDirectory is a directory.
	TDirectory
	// TSymlink is a symlink.
	TSymlink
)

func (t FileType) String() string {
	switch t {
	case TUnknown:
		return "unknown"
	case TFile:
		return "file"
	case TDirectory:
		return "directory"
	case TSymlink:
		return "symlink"
	default:
		return "<unknown file type>"
	}
}

// DirEntry is a directory entry returned by `Ls`.
type DirEntry struct {
	Name string
	Cid  cid.Cid

	// Only filled when asked to resolve the directory entry.
	Size   uint64   // The size of the file in bytes (or the size of the symlink).
	Type   FileType // The type of the file.
	Target string   // The symlink target (if a symlink).

	Mode    os.FileMode
	ModTime time.Time
}

// UnixfsAPI is the basic interface to immutable files in IPFS
// NOTE: This API is heavily WIP, things are guaranteed to break frequently
type UnixfsAPI interface {
	// Add imports the data from the reader into merkledag file
	//
	// TODO: a long useful comment on how to use this for many different scenarios
	Add(context.Context, files.Node, ...options.UnixfsAddOption) (path.ImmutablePath, error)

	// Get returns a read-only handle to a file tree referenced by a path
	//
	// Note that some implementations of this API may apply the specified context
	// to operations performed on the returned file
	Get(context.Context, path.Path) (files.Node, error)

	// Ls writes the links in a directory to the DirEntry channel. Links aren't
	// guaranteed to be returned in order. If an error occurs or the context is
	// canceled, the DirEntry channel is closed and an error is returned.
	//
	// Example:
	//
	//  dirs := make(chan DirEntry)
	//  lsErr := make(chan error, 1)
	//  go func() {
	//		lsErr <- Ls(ctx, p, dirs)
	//  }()
	//	for dirEnt := range dirs {
	//		fmt.Println("Dir name:", dirEnt.Name)
	//	}
	//	err := <-lsErr
	//	if err != nil {
	//		return fmt.Errorf("error listing directory: %w", err)
	//	}
	Ls(context.Context, path.Path, chan<- DirEntry, ...options.UnixfsLsOption) error
}

// LsIter returns a go iterator that allows ranging over DirEntry results.
// Iteration stops if the context is canceled or if the iterator yields an
// error.
//
// Example:
//
//	for dirEnt, err := LsIter(ctx, ufsAPI, p) {
//		if err != nil {
//			return fmt.Errorf("error listing directory: %w", err)
//		}
//		fmt.Println("Dir name:", dirEnt.Name)
//	}
func LsIter(ctx context.Context, api UnixfsAPI, p path.Path, opts ...options.UnixfsLsOption) iter.Seq2[DirEntry, error] {
	return func(yield func(DirEntry, error) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel() // cancel Ls if done iterating early

		dirs := make(chan DirEntry)
		lsErr := make(chan error, 1)
		go func() {
			lsErr <- api.Ls(ctx, p, dirs, opts...)
		}()
		for dirEnt := range dirs {
			if !yield(dirEnt, nil) {
				return
			}
		}
		if err := <-lsErr; err != nil {
			yield(DirEntry{}, err)
		}
	}
}
