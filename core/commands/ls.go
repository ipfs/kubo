package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	cmdenv "github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	unixfs "github.com/ipfs/boxo/ipld/unixfs"
	unixfs_pb "github.com/ipfs/boxo/ipld/unixfs/pb"
	cmds "github.com/ipfs/go-ipfs-cmds"
	iface "github.com/ipfs/kubo/core/coreiface"
	options "github.com/ipfs/kubo/core/coreiface/options"
)

// LsLink contains printable data for a single ipld link in ls output
type LsLink struct {
	Name, Hash string
	Size       uint64
	Type       unixfs_pb.Data_DataType
	Target     string
	Mode       os.FileMode
	ModTime    time.Time
}

// LsObject is an element of LsOutput
// It can represent all or part of a directory
type LsObject struct {
	Hash  string
	Links []LsLink
}

// LsOutput is a set of printable data for directories,
// it can be complete or partial
type LsOutput struct {
	Objects []LsObject
}

const (
	lsHeadersOptionNameTime = "headers"
	lsResolveTypeOptionName = "resolve-type"
	lsSizeOptionName        = "size"
	lsStreamOptionName      = "stream"
	lsLongOptionName        = "long"
)

var LsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List directory contents for Unix filesystem objects.",
		ShortDescription: `
Displays the contents of an IPFS or IPNS object(s) at the given path, with
the following format:

  <cid> <size> <name>

With the --long (-l) option, display optional file mode (permissions) and
modification time in a format similar to Unix 'ls -l':

  <mode> <cid> <size> <mtime> <name>

Mode and mtime are optional UnixFS metadata. They are only present if the
content was imported with 'ipfs add --preserve-mode' and '--preserve-mtime'.
Without preserved metadata, both mode and mtime display '-'. Times are in UTC.

Example with --long and preserved metadata:

  -rw-r--r-- QmZULkCELmmk5XNf... 1234 Jan 15 10:30 document.txt
  -rwxr-xr-x QmaRGe7bVmVaLmxb... 5678 Dec 01  2023 script.sh
  drwxr-xr-x QmWWEQhcLufF3qPm... -    Nov 20  2023 subdir/

Example with --long without preserved metadata:

  -          QmZULkCELmmk5XNf... 1234 -            document.txt

The JSON output contains type information.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "The path to the IPFS object(s) to list links from.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(lsHeadersOptionNameTime, "v", "Print table headers (Hash, Size, Name)."),
		cmds.BoolOption(lsResolveTypeOptionName, "Resolve linked objects to find out their types.").WithDefault(true),
		cmds.BoolOption(lsSizeOptionName, "Resolve linked objects to find out their file size.").WithDefault(true),
		cmds.BoolOption(lsStreamOptionName, "s", "Enable experimental streaming of directory entries as they are traversed."),
		cmds.BoolOption(lsLongOptionName, "l", "Use a long listing format, showing file mode and modification time."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		resolveType, _ := req.Options[lsResolveTypeOptionName].(bool)
		resolveSize, _ := req.Options[lsSizeOptionName].(bool)
		stream, _ := req.Options[lsStreamOptionName].(bool)

		err = req.ParseBodyArgs()
		if err != nil {
			return err
		}
		paths := req.Arguments

		enc, err := cmdenv.GetCidEncoder(req)
		if err != nil {
			return err
		}

		var processLink func(path string, link LsLink) error
		var dirDone func(i int)

		processDir := func() (func(path string, link LsLink) error, func(i int)) {
			return func(path string, link LsLink) error {
				output := []LsObject{{
					Hash:  path,
					Links: []LsLink{link},
				}}
				return res.Emit(&LsOutput{output})
			}, func(i int) {}
		}
		done := func() error { return nil }

		if !stream {
			output := make([]LsObject, len(req.Arguments))

			processDir = func() (func(path string, link LsLink) error, func(i int)) {
				// for each dir
				outputLinks := make([]LsLink, 0)
				return func(path string, link LsLink) error {
						// for each link
						outputLinks = append(outputLinks, link)
						return nil
					}, func(i int) {
						// after each dir
						slices.SortFunc(outputLinks, func(a, b LsLink) int {
							return strings.Compare(a.Name, b.Name)
						})

						output[i] = LsObject{
							Hash:  paths[i],
							Links: outputLinks,
						}
					}
			}

			done = func() error {
				return cmds.EmitOnce(res, &LsOutput{output})
			}
		}

		lsCtx, cancel := context.WithCancel(req.Context)
		defer cancel()

		for i, fpath := range paths {
			pth, err := cmdutils.PathOrCidPath(fpath)
			if err != nil {
				return err
			}

			results := make(chan iface.DirEntry)
			lsErr := make(chan error, 1)
			go func() {
				lsErr <- api.Unixfs().Ls(lsCtx, pth, results,
					options.Unixfs.ResolveChildren(resolveSize || resolveType))
			}()

			processLink, dirDone = processDir()
			for link := range results {
				var ftype unixfs_pb.Data_DataType
				switch link.Type {
				case iface.TFile:
					ftype = unixfs.TFile
				case iface.TDirectory:
					ftype = unixfs.TDirectory
				case iface.TSymlink:
					ftype = unixfs.TSymlink
				}
				lsLink := LsLink{
					Name: link.Name,
					Hash: enc.Encode(link.Cid),

					Size:   link.Size,
					Type:   ftype,
					Target: link.Target,

					Mode:    link.Mode,
					ModTime: link.ModTime,
				}
				if err = processLink(paths[i], lsLink); err != nil {
					return err
				}
			}
			if err = <-lsErr; err != nil {
				return err
			}
			dirDone(i)
		}
		return done()
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			req := res.Request()
			lastObjectHash := ""

			for {
				v, err := res.Next()
				if err != nil {
					if err == io.EOF {
						return nil
					}
					return err
				}
				out := v.(*LsOutput)
				lastObjectHash = tabularOutput(req, os.Stdout, out, lastObjectHash, false)
			}
		},
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *LsOutput) error {
			// when streaming over HTTP using a text encoder, we cannot render breaks
			// between directories because we don't know the hash of the last
			// directory encoder
			ignoreBreaks, _ := req.Options[lsStreamOptionName].(bool)
			tabularOutput(req, w, out, "", ignoreBreaks)
			return nil
		}),
	},
	Type: LsOutput{},
}

// formatMode converts os.FileMode to a 10-character Unix ls-style string.
//
// Format: [type][owner rwx][group rwx][other rwx]
//
// Type indicators: - (regular), d (directory), l (symlink), p (named pipe),
// s (socket), c (char device), b (block device).
//
// Special bits replace the execute position: setuid on owner (s/S),
// setgid on group (s/S), sticky on other (t/T). Lowercase when the
// underlying execute bit is also set, uppercase when not.
func formatMode(mode os.FileMode) string {
	var buf [10]byte

	// File type - handle all special file types like ls does
	switch {
	case mode&os.ModeDir != 0:
		buf[0] = 'd'
	case mode&os.ModeSymlink != 0:
		buf[0] = 'l'
	case mode&os.ModeNamedPipe != 0:
		buf[0] = 'p'
	case mode&os.ModeSocket != 0:
		buf[0] = 's'
	case mode&os.ModeDevice != 0:
		if mode&os.ModeCharDevice != 0 {
			buf[0] = 'c'
		} else {
			buf[0] = 'b'
		}
	default:
		buf[0] = '-'
	}

	// Owner permissions (bits 8,7,6)
	buf[1] = permBit(mode, 0400, 'r') // read
	buf[2] = permBit(mode, 0200, 'w') // write
	// Handle setuid bit for owner execute
	if mode&os.ModeSetuid != 0 {
		if mode&0100 != 0 {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	} else {
		buf[3] = permBit(mode, 0100, 'x') // execute
	}

	// Group permissions (bits 5,4,3)
	buf[4] = permBit(mode, 0040, 'r') // read
	buf[5] = permBit(mode, 0020, 'w') // write
	// Handle setgid bit for group execute
	if mode&os.ModeSetgid != 0 {
		if mode&0010 != 0 {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	} else {
		buf[6] = permBit(mode, 0010, 'x') // execute
	}

	// Other permissions (bits 2,1,0)
	buf[7] = permBit(mode, 0004, 'r') // read
	buf[8] = permBit(mode, 0002, 'w') // write
	// Handle sticky bit for other execute
	if mode&os.ModeSticky != 0 {
		if mode&0001 != 0 {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	} else {
		buf[9] = permBit(mode, 0001, 'x') // execute
	}

	return string(buf[:])
}

// permBit returns the permission character if the bit is set.
func permBit(mode os.FileMode, bit os.FileMode, char byte) byte {
	if mode&bit != 0 {
		return char
	}
	return '-'
}

// formatModTime formats time.Time for display, following Unix ls conventions.
//
// Returns "-" for zero time. Otherwise returns a 12-character string:
// recent files (within 6 months) show "Jan 02 15:04",
// older or future files show "Jan 02  2006".
//
// The output uses the timezone embedded in t (UTC for IPFS metadata).
func formatModTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	// Format: "Jan 02 15:04" for times within the last 6 months
	// Format: "Jan 02  2006" for older times (similar to ls)
	now := time.Now()
	sixMonthsAgo := now.AddDate(0, -6, 0)

	if t.After(sixMonthsAgo) && t.Before(now.Add(24*time.Hour)) {
		return t.Format("Jan 02 15:04")
	}
	return t.Format("Jan 02  2006")
}

func tabularOutput(req *cmds.Request, w io.Writer, out *LsOutput, lastObjectHash string, ignoreBreaks bool) string {
	headers, _ := req.Options[lsHeadersOptionNameTime].(bool)
	stream, _ := req.Options[lsStreamOptionName].(bool)
	size, _ := req.Options[lsSizeOptionName].(bool)
	long, _ := req.Options[lsLongOptionName].(bool)

	// in streaming mode we can't automatically align the tabs
	// so we take a best guess
	var minTabWidth int
	if stream {
		minTabWidth = 10
	} else {
		minTabWidth = 1
	}

	multipleFolders := len(req.Arguments) > 1

	tw := tabwriter.NewWriter(w, minTabWidth, 2, 1, ' ', 0)

	for _, object := range out.Objects {

		if !ignoreBreaks && object.Hash != lastObjectHash {
			if multipleFolders {
				if lastObjectHash != "" {
					fmt.Fprintln(tw)
				}
				fmt.Fprintf(tw, "%s:\n", object.Hash)
			}
			if headers {
				var s string
				if long {
					// Long format: Mode Hash [Size] ModTime Name
					if size {
						s = "Mode\tHash\tSize\tModTime\tName"
					} else {
						s = "Mode\tHash\tModTime\tName"
					}
				} else {
					// Standard format: Hash [Size] Name
					if size {
						s = "Hash\tSize\tName"
					} else {
						s = "Hash\tName"
					}
				}
				fmt.Fprintln(tw, s)
			}
			lastObjectHash = object.Hash
		}

		for _, link := range object.Links {
			var s string
			isDir := link.Type == unixfs.TDirectory || link.Type == unixfs.THAMTShard || link.Type == unixfs.TMetadata

			if long {
				// Long format: Mode Hash Size ModTime Name
				var mode string
				if link.Mode == 0 {
					// No mode metadata preserved. Show "-" to indicate
					// "not available" rather than "----------" (mode 0000).
					mode = "-"
				} else {
					mode = formatMode(link.Mode)
				}
				modTime := formatModTime(link.ModTime)

				if isDir {
					if size {
						s = "%s\t%s\t-\t%s\t%s/\n"
					} else {
						s = "%s\t%s\t%s\t%s/\n"
					}
					fmt.Fprintf(tw, s, mode, link.Hash, modTime, cmdenv.EscNonPrint(link.Name))
				} else {
					if size {
						s = "%s\t%s\t%v\t%s\t%s\n"
						fmt.Fprintf(tw, s, mode, link.Hash, link.Size, modTime, cmdenv.EscNonPrint(link.Name))
					} else {
						s = "%s\t%s\t%s\t%s\n"
						fmt.Fprintf(tw, s, mode, link.Hash, modTime, cmdenv.EscNonPrint(link.Name))
					}
				}
			} else {
				// Standard format: Hash [Size] Name
				switch {
				case isDir:
					if size {
						s = "%[1]s\t-\t%[3]s/\n"
					} else {
						s = "%[1]s\t%[3]s/\n"
					}
				default:
					if size {
						s = "%s\t%v\t%s\n"
					} else {
						s = "%[1]s\t%[3]s\n"
					}
				}
				fmt.Fprintf(tw, s, link.Hash, link.Size, cmdenv.EscNonPrint(link.Name))
			}
		}
	}
	tw.Flush()
	return lastObjectHash
}
