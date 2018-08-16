package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	cid "gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	mhash "gx/ipfs/QmPnFwZ2JXKnXgMw8CdBPxn7FWh6LLdjUjxV1fKHuJnkr8/go-multihash"
	cidutil "gx/ipfs/QmQJSeE3CX4zos9qeaG8EhecEK9zvrTEfTG84J8C5NVRwt/go-cidutil"
	cmdkit "gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	verifcid "gx/ipfs/QmVkMRSkXrpjqrroEXWuYBvDBnXCdMMY6gsKicBGVGUqKT/go-verifcid"
	cmds "gx/ipfs/QmXTmUCBtDUrzDYVzASogLiNph7EBuYqEgPL7QoHNMzUnz/go-ipfs-cmds"
	mbase "gx/ipfs/QmekxXDhCxCJRNuzmHreuaT3BsuJcsjcXWNrtV9C8DRHtd/go-multibase"
)

var CidCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Convert and discover properties of CIDs",
	},
	Subcommands: map[string]*cmds.Command{
		"format": cidFmtCmd,
		"base32": base32Cmd,
		"bases":  basesCmd,
		"codecs": codecsCmd,
		"hashes": hashesCmd,
	},
}

var cidFmtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Format and convert a CID in various useful ways.",
		LongDescription: `
Format and converts <cid>'s in various useful ways.

The optional format string either "prefix" or or a printf style format string:
` + cidutil.FormatRef,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, true, "Cids to format."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("f", "Format string."),
		cmdkit.StringOption("v", "CID version to convert to."),
		cmdkit.StringOption("b", "Multibase to display CID in."),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		fmtStr, _ := req.Options["f"].(string)
		verStr, _ := req.Options["v"].(string)
		baseStr, _ := req.Options["b"].(string)

		opts := cidFormatOpts{}

		switch fmtStr {
		case "":
			opts.fmtStr = "%s"
		case "prefix":
			opts.fmtStr = "%P"
		default:
			if strings.IndexByte(fmtStr, '%') == -1 {
				return fmt.Errorf("invalid format string: %s", fmtStr)
			}
			opts.fmtStr = fmtStr
		}

		switch verStr {
		case "":
			// noop
		case "0":
			opts.verConv = toCidV0
		case "1":
			opts.verConv = toCidV1
		default:
			return fmt.Errorf("invalid cid version: %s\n", verStr)
		}

		if baseStr != "" {
			encoder, err := mbase.EncoderByName(baseStr)
			if err != nil {
				return err
			}
			opts.newBase = encoder.Encoding()
		} else {
			opts.newBase = mbase.Encoding(-1)
		}

		res, err := formatCids(req.Arguments, opts)
		if err != nil {
			return err
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, val interface{}) error {
			for _, v := range val.([]string) {
				fmt.Fprintf(w, "%s\n", v)
			}
			return nil
		}),
	},
	Type: []string{},
}

var base32Cmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Convert CIDs to Base32 CID version 1.",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("cid", true, true, "Cids to convert."),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		opts := cidFormatOpts{
			fmtStr:  "%s",
			newBase: mbase.Encoding(mbase.Base32),
			verConv: toCidV1,
		}
		res, err := formatCids(req.Arguments, opts)
		if err != nil {
			return err
		}

		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cidFmtCmd.Encoders,
	Type:     cidFmtCmd.Type,
}

type cidFormatOpts struct {
	fmtStr  string
	newBase mbase.Encoding
	verConv func(cid cid.Cid) (cid.Cid, error)
}

func formatCids(args []string, opts cidFormatOpts) ([]string, error) {
	var res []string
	for _, cidStr := range args {
		c, err := cid.Decode(cidStr)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", cidStr, err)
		}
		base := opts.newBase
		if base == -1 {
			base, _ = cid.ExtractEncoding(cidStr)
		}
		if opts.verConv != nil {
			c, err = opts.verConv(c)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", cidStr, err)
			}
		}
		str, err := cidutil.Format(opts.fmtStr, base, c)
		if _, ok := err.(cidutil.FormatStringError); ok {
			return nil, err
		} else if err != nil {
			return nil, fmt.Errorf("%s: %v", cidStr, err)
		}
		res = append(res, str)
	}
	return res, nil
}

func toCidV0(c cid.Cid) (cid.Cid, error) {
	if c.Type() != cid.DagProtobuf {
		return cid.Cid{}, fmt.Errorf("can't convert non-protobuf nodes to cidv0")
	}
	return cid.NewCidV0(c.Hash()), nil
}

func toCidV1(c cid.Cid) (cid.Cid, error) {
	return cid.NewCidV1(c.Type(), c.Hash()), nil
}

type CodeAndName struct {
	Code int
	Name string
}

var basesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available multibase encodings.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("prefix", "also include the single leter prefixes in addition to the code"),
		cmdkit.BoolOption("numeric", "also include numeric codes"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use EncodingToStr in case at some point there are multiple names for a given code
		for code, name := range mbase.EncodingToStr {
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w0 io.Writer, val0 interface{}) error {
			w := tabwriter.NewWriter(w0, 0, 0, 2, ' ', 0)
			prefixes, _ := req.Options["prefix"].(bool)
			numeric, _ := req.Options["numeric"].(bool)
			val := val0.([]CodeAndName)
			sort.Sort(multibaseSorter{val})
			for _, v := range val {
				if prefixes && v.Code >= 32 && v.Code < 127 {
					fmt.Fprintf(w, "%c\t", v.Code)
				} else if prefixes {
					// don't display non-printable prefixes
					fmt.Fprintf(w, "\t")
				}
				if numeric {
					fmt.Fprintf(w, "%d\t%s\n", v.Code, v.Name)
				} else {
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			w.Flush()
			return nil
		}),
	},
	Type: []CodeAndName{},
}

var codecsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available CID codecs.",
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("numeric", "also include numeric codes"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use CodecToStr as there are multiple names for a given code
		for code, name := range cid.CodecToStr {
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w0 io.Writer, val0 interface{}) error {
			w := tabwriter.NewWriter(w0, 0, 0, 2, ' ', 0)
			numeric, _ := req.Options["numeric"].(bool)
			val := val0.([]CodeAndName)
			sort.Sort(codeAndNameSorter{val})
			for _, v := range val {
				if numeric {
					fmt.Fprintf(w, "%d\t%s\n", v.Code, v.Name)
				} else {
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			w.Flush()
			return nil
		}),
	},
	Type: []CodeAndName{},
}

var hashesCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "List available multihashes.",
	},
	Options: codecsCmd.Options,
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use mhash.Codes in case at some point there are multiple names for a given code
		for code, name := range mhash.Codes {
			if !verifcid.IsGoodHash(code) {
				continue
			}
			res = append(res, CodeAndName{int(code), name})
		}
		cmds.EmitOnce(resp, res)
		return nil
	},
	Encoders: codecsCmd.Encoders,
	Type:     codecsCmd.Type,
}

type multibaseSorter struct {
	data []CodeAndName
}

func (s multibaseSorter) Len() int      { return len(s.data) }
func (s multibaseSorter) Swap(i, j int) { s.data[i], s.data[j] = s.data[j], s.data[i] }

func (s multibaseSorter) Less(i, j int) bool {
	a := unicode.ToLower(rune(s.data[i].Code))
	b := unicode.ToLower(rune(s.data[j].Code))
	if a != b {
		return a < b
	}
	// lowecase letters should come before uppercase
	return s.data[i].Code > s.data[j].Code
}

type codeAndNameSorter struct {
	data []CodeAndName
}

func (s codeAndNameSorter) Len() int           { return len(s.data) }
func (s codeAndNameSorter) Swap(i, j int)      { s.data[i], s.data[j] = s.data[j], s.data[i] }
func (s codeAndNameSorter) Less(i, j int) bool { return s.data[i].Code < s.data[j].Code }
