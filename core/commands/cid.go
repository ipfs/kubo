package commands

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	verifcid "github.com/ipfs/boxo/verifcid"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipldmulticodec "github.com/ipld/go-ipld-prime/multicodec"
	mbase "github.com/multiformats/go-multibase"
	mc "github.com/multiformats/go-multicodec"
	mhash "github.com/multiformats/go-multihash"
)

var CidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Convert and discover properties of CIDs",
	},
	Subcommands: map[string]*cmds.Command{
		"format": cidFmtCmd,
		"base32": base32Cmd,
		"bases":  basesCmd,
		"codecs": codecsCmd,
		"hashes": hashesCmd,
	},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
}

const (
	cidFormatOptionName    = "f"
	cidVerisonOptionName   = "v"
	cidCodecOptionName     = "mc"
	cidMultibaseOptionName = "b"
)

var cidFmtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Format and convert a CID in various useful ways.",
		LongDescription: `
Format and converts <cid>'s in various useful ways.

The optional format string is a printf style format string:
` + cidutil.FormatRef,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CIDs to format.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(cidFormatOptionName, "Printf style format string.").WithDefault("%s"),
		cmds.StringOption(cidVerisonOptionName, "CID version to convert to."),
		cmds.StringOption(cidCodecOptionName, "CID multicodec to convert to."),
		cmds.StringOption(cidMultibaseOptionName, "Multibase to display CID in."),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		fmtStr, _ := req.Options[cidFormatOptionName].(string)
		verStr, _ := req.Options[cidVerisonOptionName].(string)
		codecStr, _ := req.Options[cidCodecOptionName].(string)
		baseStr, _ := req.Options[cidMultibaseOptionName].(string)

		opts := cidFormatOpts{}

		if strings.IndexByte(fmtStr, '%') == -1 {
			return fmt.Errorf("invalid format string: %q", fmtStr)
		}
		opts.fmtStr = fmtStr

		if codecStr != "" {
			var codec mc.Code
			err := codec.Set(codecStr)
			if err != nil {
				return err
			}
			opts.newCodec = uint64(codec)
		} // otherwise, leave it as 0 (not a valid IPLD codec)

		switch verStr {
		case "":
			if baseStr != "" {
				opts.verConv = toCidV1
			}
		case "0":
			if opts.newCodec != 0 && opts.newCodec != cid.DagProtobuf {
				return errors.New("cannot convert to CIDv0 with any codec other than dag-pb")
			}
			if baseStr != "" && baseStr != "base58btc" {
				return errors.New("cannot convert to CIDv0 with any multibase other than the implicit base58btc")
			}
			opts.verConv = toCidV0
		case "1":
			opts.verConv = toCidV1
		default:
			return fmt.Errorf("invalid cid version: %q", verStr)
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

		return emitCids(req, resp, opts)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: streamResult(func(v interface{}, out io.Writer) nonFatalError {
			r := v.(*CidFormatRes)
			if r.ErrorMsg != "" {
				return nonFatalError(fmt.Sprintf("%s: %s", r.CidStr, r.ErrorMsg))
			}
			fmt.Fprintf(out, "%s\n", r.Formatted)
			return ""
		}),
	},
	Type: CidFormatRes{},
}

type CidFormatRes struct {
	CidStr    string // Original Cid String passed in
	Formatted string // Formatted Result
	ErrorMsg  string // Error
}

var base32Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Convert CIDs to Base32 CID version 1.",
		ShortDescription: `
'ipfs cid base32' normalizes passed CIDs to their canonical case-insensitive encoding.
Useful when processing third-party CIDs which could come with arbitrary formats.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CIDs to convert.").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		opts := cidFormatOpts{
			fmtStr:  "%s",
			newBase: mbase.Encoding(mbase.Base32),
			verConv: toCidV1,
		}
		return emitCids(req, resp, opts)
	},
	PostRun: cidFmtCmd.PostRun,
	Type:    cidFmtCmd.Type,
}

type cidFormatOpts struct {
	fmtStr   string
	newBase  mbase.Encoding
	verConv  func(cid cid.Cid) (cid.Cid, error)
	newCodec uint64
}

type argumentIterator struct {
	args []string
	body cmds.StdinArguments
}

func (i *argumentIterator) next() (string, bool) {
	if len(i.args) > 0 {
		arg := i.args[0]
		i.args = i.args[1:]
		return arg, true
	}
	if i.body == nil || !i.body.Scan() {
		return "", false
	}
	return strings.TrimSpace(i.body.Argument()), true
}

func (i *argumentIterator) err() error {
	if i.body == nil {
		return nil
	}
	return i.body.Err()
}

func emitCids(req *cmds.Request, resp cmds.ResponseEmitter, opts cidFormatOpts) error {
	itr := argumentIterator{req.Arguments, req.BodyArgs()}
	var emitErr error
	for emitErr == nil {
		cidStr, ok := itr.next()
		if !ok {
			break
		}
		res := &CidFormatRes{CidStr: cidStr}
		c, err := cid.Decode(cidStr)
		if err != nil {
			res.ErrorMsg = err.Error()
			emitErr = resp.Emit(res)
			continue
		}

		if opts.newCodec != 0 && opts.newCodec != c.Type() {
			c = cid.NewCidV1(opts.newCodec, c.Hash())
		}

		if opts.verConv != nil {
			c, err = opts.verConv(c)
			if err != nil {
				res.ErrorMsg = err.Error()
				emitErr = resp.Emit(res)
				continue
			}
		}

		base := opts.newBase
		if base == -1 {
			if c.Version() == 0 {
				base = mbase.Base58BTC
			} else {
				base, _ = cid.ExtractEncoding(cidStr)
			}
		}

		str, err := cidutil.Format(opts.fmtStr, base, c)
		if _, ok := err.(cidutil.FormatStringError); ok {
			// no point in continuing if there is a problem with the format string
			return err
		}
		if err != nil {
			res.ErrorMsg = err.Error()
		} else {
			res.Formatted = str
		}
		emitErr = resp.Emit(res)
	}
	if emitErr != nil {
		return emitErr
	}
	err := itr.err()
	if err != nil {
		return err
	}
	return nil
}

func toCidV0(c cid.Cid) (cid.Cid, error) {
	if c.Type() != cid.DagProtobuf {
		return cid.Cid{}, fmt.Errorf("can't convert non-dag-pb nodes to cidv0")
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

const (
	prefixOptionName  = "prefix"
	numericOptionName = "numeric"
)

var basesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List available multibase encodings.",
		ShortDescription: `
'ipfs cid bases' relies on https://github.com/multiformats/go-multibase
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(prefixOptionName, "also include the single letter prefixes in addition to the code"),
		cmds.BoolOption(numericOptionName, "also include numeric codes"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use EncodingToStr in case at some point there are multiple names for a given code
		for code, name := range mbase.EncodingToStr {
			res = append(res, CodeAndName{int(code), name})
		}
		return cmds.EmitOnce(resp, res)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val []CodeAndName) error {
			prefixes, _ := req.Options[prefixOptionName].(bool)
			numeric, _ := req.Options[numericOptionName].(bool)
			sort.Sort(multibaseSorter{val})
			for _, v := range val {
				code := v.Code
				if code < 32 || code >= 127 {
					// don't display non-printable prefixes
					code = ' '
				}
				switch {
				case prefixes && numeric:
					fmt.Fprintf(w, "%c %7d  %s\n", code, v.Code, v.Name)
				case prefixes:
					fmt.Fprintf(w, "%c  %s\n", code, v.Name)
				case numeric:
					fmt.Fprintf(w, "%7d  %s\n", v.Code, v.Name)
				default:
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			return nil
		}),
	},
	Type: []CodeAndName{},
}

const (
	codecsNumericOptionName   = "numeric"
	codecsSupportedOptionName = "supported"
)

var codecsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List available CID multicodecs.",
		ShortDescription: `
'ipfs cid codecs' relies on https://github.com/multiformats/go-multicodec
`,
	},
	Options: []cmds.Option{
		cmds.BoolOption(codecsNumericOptionName, "n", "also include numeric codes"),
		cmds.BoolOption(codecsSupportedOptionName, "s", "list only codecs supported by go-ipfs commands"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		listSupported, _ := req.Options[codecsSupportedOptionName].(bool)
		supportedCodecs := make(map[uint64]struct{})
		if listSupported {
			for _, code := range ipldmulticodec.ListEncoders() {
				supportedCodecs[code] = struct{}{}
			}
			for _, code := range ipldmulticodec.ListDecoders() {
				supportedCodecs[code] = struct{}{}
			}
			// add libp2p-key
			supportedCodecs[uint64(mc.Libp2pKey)] = struct{}{}
		}

		var res []CodeAndName
		for _, code := range mc.KnownCodes() {
			if code.Tag() == "ipld" {
				if listSupported {
					if _, ok := supportedCodecs[uint64(code)]; !ok {
						continue
					}
				}
				res = append(res, CodeAndName{int(code), mc.Code(code).String()})
			}
		}
		return cmds.EmitOnce(resp, res)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, val []CodeAndName) error {
			numeric, _ := req.Options[codecsNumericOptionName].(bool)
			sort.Sort(codeAndNameSorter{val})
			for _, v := range val {
				if numeric {
					fmt.Fprintf(w, "%5d  %s\n", v.Code, v.Name)
				} else {
					fmt.Fprintf(w, "%s\n", v.Name)
				}
			}
			return nil
		}),
	},
	Type: []CodeAndName{},
}

var hashesCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List available multihashes.",
		ShortDescription: `
'ipfs cid hashes' relies on https://github.com/multiformats/go-multihash
`,
	},
	Options: codecsCmd.Options,
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		var res []CodeAndName
		// use mhash.Codes in case at some point there are multiple names for a given code
		for code, name := range mhash.Codes {
			if !verifcid.DefaultAllowlist.IsAllowed(code) {
				continue
			}
			res = append(res, CodeAndName{int(code), name})
		}
		return cmds.EmitOnce(resp, res)
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
