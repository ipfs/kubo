package commands

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	verifcid "github.com/ipfs/boxo/verifcid"
	cid "github.com/ipfs/go-cid"
	cidutil "github.com/ipfs/go-cidutil"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipldmulticodec "github.com/ipld/go-ipld-prime/multicodec"
	peer "github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
	mc "github.com/multiformats/go-multicodec"
	mhash "github.com/multiformats/go-multihash"
)

var CidCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Convert and discover properties of CIDs",
	},
	Subcommands: map[string]*cmds.Command{
		"inspect": inspectCmd,
		"format":  cidFmtCmd,
		"base32":  base32Cmd,
		"bases":   basesCmd,
		"codecs":  codecsCmd,
		"hashes":  hashesCmd,
	},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
}

const (
	cidFormatOptionName    = "f"
	cidToVersionOptionName = "v"
	cidCodecOptionName     = "mc"
	cidMultibaseOptionName = "b"
)

var cidFmtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Format and convert a CID in various useful ways.",
		LongDescription: `
Format and converts <cid>'s in various useful ways.

For a human-readable breakdown of a CID, see 'ipfs cid inspect'.

The optional format string is a printf style format string:
` + cidutil.FormatRef,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, true, "CIDs to format.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(cidFormatOptionName, "Printf style format string.").WithDefault("%s"),
		cmds.StringOption(cidToVersionOptionName, "CID version to convert to."),
		cmds.StringOption(cidCodecOptionName, "CID multicodec to convert to."),
		cmds.StringOption(cidMultibaseOptionName, "Multibase to display CID in."),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		fmtStr, _ := req.Options[cidFormatOptionName].(string)
		verStr, _ := req.Options[cidToVersionOptionName].(string)
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
		cmds.CLI: streamResult(func(v any, out io.Writer) nonFatalError {
			r := v.(*CidFormatRes)
			if r.ErrorMsg != "" {
				return nonFatalError(fmt.Sprintf("%s: %s", r.CidStr, r.ErrorMsg))
			}
			fmt.Fprintf(out, "%s\n", r.Formatted)
			return ""
		}),
	},
	Type:  CidFormatRes{},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
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
	Extra:   CreateCmdExtras(SetDoesNotUseRepo(true)),
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
			multibaseSorter{val}.Sort()
			for _, v := range val {
				code := v.Code
				if !unicode.IsPrint(rune(code)) {
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
	Type:  []CodeAndName{},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
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
			codeAndNameSorter{val}.Sort()
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
	Type:  []CodeAndName{},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
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
	Extra:    CreateCmdExtras(SetDoesNotUseRepo(true)),
}

// CidInspectRes represents the response from the inspect command.
type CidInspectRes struct {
	Cid        string          `json:"cid"`
	Version    int             `json:"version"`
	Multibase  CidInspectBase  `json:"multibase"`
	Multicodec CidInspectCodec `json:"multicodec"`
	Multihash  CidInspectHash  `json:"multihash"`
	CidV0      string          `json:"cidV0,omitempty"`
	CidV1      string          `json:"cidV1"`
	ErrorMsg   string          `json:"errorMsg,omitempty"`
}

type CidInspectBase struct {
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
}

type CidInspectCodec struct {
	Code uint64 `json:"code"`
	Name string `json:"name"`
}

type CidInspectHash struct {
	Code   uint64 `json:"code"`
	Name   string `json:"name"`
	Length int    `json:"length"`
	Digest string `json:"digest"`
}

var inspectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Inspect and display detailed information about a CID.",
		ShortDescription: `
'ipfs cid inspect' breaks down a CID and displays its components:
- CID version (0 or 1)
- Multibase encoding (explicit for CIDv1, implicit for CIDv0)
- Multicodec (DAG type)
- Multihash (hash algorithm, length, and digest)
- Equivalent CIDv0 and CIDv1 representations

For CIDv0, multibase, multicodec, and multihash are marked as
implicit because they are not explicitly encoded in the binary.

If a PeerID string is provided instead of a CID, a helpful error
with the equivalent CID representation is returned.

Use --enc=json for machine-readable output same as the HTTP RPC API.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "CID to inspect.").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		cidStr := req.Arguments[0]

		c, err := cid.Decode(cidStr)
		if err != nil {
			errMsg := fmt.Sprintf("invalid CID: %s", err)
			// PeerID fallback: try peer.Decode for legacy PeerIDs (12D3KooW..., Qm...)
			if pid, pidErr := peer.Decode(cidStr); pidErr == nil {
				pidCid := peer.ToCid(pid)
				cidV1, _ := pidCid.StringOfBase(mbase.Base36)
				errMsg += fmt.Sprintf("\nNote: the value is a PeerID; inspect its CID representation instead:\n  %s", cidV1)
			}
			return cmds.EmitOnce(resp, &CidInspectRes{Cid: cidStr, ErrorMsg: errMsg})
		}

		res := &CidInspectRes{
			Cid:     cidStr,
			Version: int(c.Version()),
		}

		// Multibase: always populated; CIDv0 uses implicit base58btc
		if c.Version() == 0 {
			res.Multibase = CidInspectBase{Prefix: "z", Name: "base58btc"}
		} else {
			baseCode, _ := cid.ExtractEncoding(cidStr)
			res.Multibase = CidInspectBase{
				Prefix: string(rune(baseCode)),
				Name:   mbase.EncodingToStr[baseCode],
			}
		}

		// Multicodec
		codecName := mc.Code(c.Type()).String()
		if codecName == "" || strings.HasPrefix(codecName, "Code(") {
			codecName = "unknown"
		}
		res.Multicodec = CidInspectCodec{Code: c.Type(), Name: codecName}

		// Multihash
		dmh, err := mhash.Decode(c.Hash())
		if err != nil {
			return cmds.EmitOnce(resp, &CidInspectRes{
				Cid:      cidStr,
				ErrorMsg: fmt.Sprintf("failed to decode multihash: %s", err),
			})
		}
		hashName := mhash.Codes[dmh.Code]
		if hashName == "" {
			hashName = "unknown"
		}
		res.Multihash = CidInspectHash{
			Code:   dmh.Code,
			Name:   hashName,
			Length: dmh.Length,
			Digest: hex.EncodeToString(dmh.Digest),
		}

		// CIDv0: only possible with dag-pb + sha2-256-256
		if c.Type() == cid.DagProtobuf && dmh.Code == mhash.SHA2_256 && dmh.Length == 32 {
			res.CidV0 = cid.NewCidV0(c.Hash()).String()
		}

		// CIDv1: use base36 for libp2p-key, base32 for everything else
		v1 := cid.NewCidV1(c.Type(), c.Hash())
		v1Base := mbase.Encoding(mbase.Base32)
		if c.Type() == uint64(mc.Libp2pKey) {
			v1Base = mbase.Base36
		}
		v1Str, err := v1.StringOfBase(v1Base)
		if err != nil {
			v1Str = v1.String()
		}
		res.CidV1 = v1Str

		return cmds.EmitOnce(resp, res)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, res *CidInspectRes) error {
			if res.ErrorMsg != "" {
				return fmt.Errorf("%s", res.ErrorMsg)
			}

			implicit := ""
			if res.Version == 0 {
				implicit = ", implicit"
			}

			fmt.Fprintf(w, "CID:        %s\n", res.Cid)
			fmt.Fprintf(w, "Version:    %d\n", res.Version)
			if res.Version == 0 {
				fmt.Fprintf(w, "Multibase:  %s (implicit)\n", res.Multibase.Name)
			} else {
				fmt.Fprintf(w, "Multibase:  %s (%s)\n", res.Multibase.Name, res.Multibase.Prefix)
			}
			fmt.Fprintf(w, "Multicodec: %s (0x%x%s)\n", res.Multicodec.Name, res.Multicodec.Code, implicit)
			fmt.Fprintf(w, "Multihash:  %s (0x%x%s)\n", res.Multihash.Name, res.Multihash.Code, implicit)
			fmt.Fprintf(w, "  Length:   %d bytes\n", res.Multihash.Length)
			fmt.Fprintf(w, "  Digest:   %s\n", res.Multihash.Digest)

			if res.CidV0 != "" {
				fmt.Fprintf(w, "CIDv0:      %s\n", res.CidV0)
			} else if res.Multicodec.Code != cid.DagProtobuf {
				fmt.Fprintf(w, "CIDv0:      not possible, requires dag-pb (0x70), got %s (0x%x)\n",
					res.Multicodec.Name, res.Multicodec.Code)
			} else if res.Multihash.Code != mhash.SHA2_256 {
				fmt.Fprintf(w, "CIDv0:      not possible, requires sha2-256 (0x12), got %s (0x%x)\n",
					res.Multihash.Name, res.Multihash.Code)
			} else if res.Multihash.Length != 32 {
				fmt.Fprintf(w, "CIDv0:      not possible, requires 32-byte digest, got %d\n",
					res.Multihash.Length)
			}

			fmt.Fprintf(w, "CIDv1:      %s\n", res.CidV1)

			return nil
		}),
	},
	Type:  CidInspectRes{},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
}

type multibaseSorter struct {
	data []CodeAndName
}

func (s multibaseSorter) Sort() {
	slices.SortFunc(s.data, func(a, b CodeAndName) int {
		if n := cmp.Compare(unicode.ToLower(rune(a.Code)), unicode.ToLower(rune(b.Code))); n != 0 {
			return n
		}
		// lowercase letters should come before uppercase
		return cmp.Compare(b.Code, a.Code)
	})
}

type codeAndNameSorter struct {
	data []CodeAndName
}

func (s codeAndNameSorter) Sort() {
	slices.SortFunc(s.data, func(a, b CodeAndName) int {
		return cmp.Compare(a.Code, b.Code)
	})
}
