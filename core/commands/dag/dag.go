package dagcmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path"

	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"

	cid "github.com/ipfs/go-cid"
	cidenc "github.com/ipfs/go-cidutil/cidenc"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

const (
	pinRootsOptionName = "pin-roots"
	progressOptionName = "progress"
	silentOptionName   = "silent"
	statsOptionName    = "stats"
)

// DagCmd provides a subset of commands for interacting with ipld dag objects
var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with IPLD DAG objects.",
		ShortDescription: `
'ipfs dag' is used for creating and manipulating DAG objects/hierarchies.

This subcommand is intended to deprecate and replace
the existing 'ipfs object' command moving forward.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"put":     DagPutCmd,
		"get":     DagGetCmd,
		"resolve": DagResolveCmd,
		"import":  DagImportCmd,
		"export":  DagExportCmd,
		"stat":    DagStatCmd,
	},
}

// OutputObject is the output type of 'dag put' command
type OutputObject struct {
	Cid cid.Cid
}

// ResolveOutput is the output type of 'dag resolve' command
type ResolveOutput struct {
	Cid     cid.Cid
	RemPath string
}

type CarImportStats struct {
	BlockCount      uint64
	BlockBytesCount uint64
}

// CarImportOutput is the output type of the 'dag import' commands
type CarImportOutput struct {
	Root  *RootMeta       `json:",omitempty"`
	Stats *CarImportStats `json:",omitempty"`
}

// RootMeta is the metadata for a root pinning response
type RootMeta struct {
	Cid         cid.Cid
	PinErrorMsg string
}

// DagPutCmd is a command for adding a dag node
var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a DAG node to IPFS.",
		ShortDescription: `
'ipfs dag put' accepts input from a file or stdin and parses it
into an object of the specified format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, true, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("store-codec", "Codec that the stored object will be encoded with").WithDefault("dag-cbor"),
		cmds.StringOption("input-codec", "Codec that the input object is encoded in").WithDefault("dag-json"),
		cmds.BoolOption("pin", "Pin this object when adding."),
		cmds.StringOption("hash", "Hash function to use"),
		cmdutils.AllowBigBlockOption,
	},
	Run:  dagPut,
	Type: OutputObject{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *OutputObject) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, enc.Encode(out.Cid))
			return nil
		}),
	},
}

// DagGetCmd is a command for getting a dag node from IPFS
var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a DAG node from IPFS.",
		ShortDescription: `
'ipfs dag get' fetches a DAG node from IPFS and prints it out in the specified
format.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The object to get").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("output-codec", "Format that the object will be encoded as.").WithDefault("dag-json"),
	},
	Run: dagGet,
}

// DagResolveCmd returns address of highest block within a path and a path remainder
var DagResolveCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Resolve IPLD block.",
		ShortDescription: `
'ipfs dag resolve' fetches a DAG node from IPFS, prints its address and remaining path.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("ref", true, false, "The path to resolve").EnableStdin(),
	},
	Run: dagResolve,
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ResolveOutput) error {
			var (
				enc cidenc.Encoder
				err error
			)
			switch {
			case !cmdenv.CidBaseDefined(req):
				// Not specified, check the path.
				enc, err = cmdenv.CidEncoderFromPath(req.Arguments[0])
				if err == nil {
					break
				}
				// Nope, fallback on the default.
				fallthrough
			default:
				enc, err = cmdenv.GetLowLevelCidEncoder(req)
				if err != nil {
					return err
				}
			}
			p := enc.Encode(out.Cid)
			if out.RemPath != "" {
				p = path.Join(p, out.RemPath)
			}

			fmt.Fprint(w, p)
			return nil
		}),
	},
	Type: ResolveOutput{},
}

// DagImportCmd is a command for importing a car to ipfs
var DagImportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Import the contents of .car files",
		ShortDescription: `
'ipfs dag import' imports all blocks present in supplied .car
( Content Address aRchive ) files, recursively pinning any roots
specified in the CAR file headers, unless --pin-roots is set to false.

Note:
  This command will import all blocks in the CAR file, not just those
  reachable from the specified roots. However, these other blocks will
  not be pinned and may be garbage collected later.

  The pinning of the roots happens after all car files are processed,
  permitting import of DAGs spanning multiple files.

  Pinning takes place in offline-mode exclusively, one root at a time.
  If the combination of blocks from the imported CAR files and what is
  currently present in the blockstore does not represent a complete DAG,
  pinning of that individual root will fail.

Maximum supported CAR version: 2
Specification of CAR formats: https://ipld.io/specs/transport/car/
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("path", true, true, "The path of a .car file.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pinRootsOptionName, "Pin optional roots listed in the .car headers after importing.").WithDefault(true),
		cmds.BoolOption(silentOptionName, "No output."),
		cmds.BoolOption(statsOptionName, "Output stats."),
		cmdutils.AllowBigBlockOption,
	},
	Type: CarImportOutput{},
	Run:  dagImport,
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *CarImportOutput) error {
			silent, _ := req.Options[silentOptionName].(bool)
			if silent {
				return nil
			}

			// event should have only one of `Root` or `Stats` set, not both
			if event.Root == nil {
				if event.Stats == nil {
					return fmt.Errorf("unexpected message from DAG import")
				}
				stats, _ := req.Options[statsOptionName].(bool)
				if stats {
					fmt.Fprintf(w, "Imported %d blocks (%d bytes)\n", event.Stats.BlockCount, event.Stats.BlockBytesCount)
				}
				return nil
			}

			if event.Stats != nil {
				return fmt.Errorf("unexpected message from DAG import")
			}

			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}

			if event.Root.PinErrorMsg != "" {
				return fmt.Errorf("pinning root %q FAILED: %s", enc.Encode(event.Root.Cid), event.Root.PinErrorMsg)
			}

			event.Root.PinErrorMsg = "success"

			_, err = fmt.Fprintf(
				w,
				"Pinned root\t%s\t%s\n",
				enc.Encode(event.Root.Cid),
				event.Root.PinErrorMsg,
			)
			return err
		}),
	},
}

// DagExportCmd is a command for exporting an ipfs dag to a car
var DagExportCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Streams the selected DAG as a .car stream on stdout.",
		ShortDescription: `
'ipfs dag export' fetches a DAG and streams it out as a well-formed .car file.
Note that at present only single root selections / .car files are supported.
The output of blocks happens in strict DAG-traversal, first-seen, order.
CAR file follows the CARv1 format: https://ipld.io/specs/transport/car/carv1/
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "CID of a root to recursively export").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Display progress on CLI. Defaults to true when STDERR is a TTY."),
	},
	Run: dagExport,
	PostRun: cmds.PostRunMap{
		cmds.CLI: finishCLIExport,
	},
}

// DagStat is a dag stat command response
type DagStat struct {
	Cid       cid.Cid `json:",omitempty"`
	Size      uint64  `json:",omitempty"`
	NumBlocks int64   `json:",omitempty"`
}

func (s *DagStat) String() string {
	return fmt.Sprintf("%s  %d  %d", s.Cid.String()[:20], s.Size, s.NumBlocks)
}

func (s *DagStat) MarshalJSON() ([]byte, error) {
	type Alias DagStat
	/*
		We can't rely on cid.Cid.MarshalJSON since it uses the {"/": "..."}
		format. To make the output consistent and follow the Kubo API patterns
		we use the Cid.String method
	*/
	return json.Marshal(struct {
		Cid string `json:"Cid"`
		*Alias
	}{
		Cid:   s.Cid.String(),
		Alias: (*Alias)(s),
	})
}

func (s *DagStat) UnmarshalJSON(data []byte) error {
	/*
		We can't rely on cid.Cid.UnmarshalJSON since it uses the {"/": "..."}
		format. To make the output consistent and follow the Kubo API patterns
		we use the Cid.Parse method
	*/
	type Alias DagStat
	aux := struct {
		Cid string `json:"Cid"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	Cid, err := cid.Parse(aux.Cid)
	if err != nil {
		return err
	}
	s.Cid = Cid
	return nil
}

type DagStatSummary struct {
	redundantSize uint64     `json:"-"`
	UniqueBlocks  int        `json:",omitempty"`
	TotalSize     uint64     `json:",omitempty"`
	SharedSize    uint64     `json:",omitempty"`
	Ratio         float32    `json:",omitempty"`
	DagStatsArray []*DagStat `json:"DagStats,omitempty"`
}

func (s *DagStatSummary) String() string {
	return fmt.Sprintf("Total Size: %d\nUnique Blocks: %d\nShared Size: %d\nRatio: %f", s.TotalSize, s.UniqueBlocks, s.SharedSize, s.Ratio)
}

func (s *DagStatSummary) incrementTotalSize(size uint64) {
	s.TotalSize += size
}

func (s *DagStatSummary) incrementRedundantSize(size uint64) {
	s.redundantSize += size
}

func (s *DagStatSummary) appendStats(stats *DagStat) {
	s.DagStatsArray = append(s.DagStatsArray, stats)
}

func (s *DagStatSummary) calculateSummary() {
	s.Ratio = float32(s.redundantSize) / float32(s.TotalSize)
	s.SharedSize = s.redundantSize - s.TotalSize
}

// DagStatCmd is a command for getting size information about an ipfs-stored dag
var DagStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Gets stats for a DAG.",
		ShortDescription: `
'ipfs dag stat' fetches a DAG and returns various statistics about it.
Statistics include size and number of blocks.

Note: This command skips duplicate blocks in reporting both size and the number of blocks
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, true, "CID of a DAG root to get statistics for").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(progressOptionName, "p", "Return progressive data while reading through the DAG").WithDefault(true),
	},
	Run:  dagStat,
	Type: DagStatSummary{},
	PostRun: cmds.PostRunMap{
		cmds.CLI: finishCLIStat,
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *DagStatSummary) error {
			fmt.Fprintln(w)
			csvWriter := csv.NewWriter(w)
			csvWriter.Comma = '\t'
			cidSpacing := len(event.DagStatsArray[0].Cid.String())
			header := []string{fmt.Sprintf("%-*s", cidSpacing, "CID"), fmt.Sprintf("%-15s", "Blocks"), "Size"}
			if err := csvWriter.Write(header); err != nil {
				return err
			}
			for _, dagStat := range event.DagStatsArray {
				numBlocksStr := fmt.Sprint(dagStat.NumBlocks)
				err := csvWriter.Write([]string{
					dagStat.Cid.String(),
					fmt.Sprintf("%-15s", numBlocksStr),
					fmt.Sprint(dagStat.Size),
				})
				if err != nil {
					return err
				}
			}
			csvWriter.Flush()
			fmt.Fprint(w, "\nSummary\n")
			_, err := fmt.Fprintf(
				w,
				"%v\n",
				event,
			)
			fmt.Fprint(w, "\n\n")
			return err
		}),
		cmds.JSON: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, event *DagStatSummary) error {
			return json.NewEncoder(w).Encode(event)
		},
		),
	},
}
