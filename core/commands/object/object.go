package objectcmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"

	humanize "github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/interface-go-ipfs-core/options"
	path "github.com/ipfs/interface-go-ipfs-core/path"
)

type Node struct {
	Links []Link
	Data  string
}

type Link struct {
	Name, Hash string
	Size       uint64
}

type Object struct {
	Hash  string `json:"Hash,omitempty"`
	Links []Link `json:"Links,omitempty"`
}

var ErrDataEncoding = errors.New("unknown data field encoding")

const (
	headersOptionName      = "headers"
	encodingOptionName     = "data-encoding"
	inputencOptionName     = "inputenc"
	datafieldencOptionName = "datafieldenc"
	pinOptionName          = "pin"
	quietOptionName        = "quiet"
	humanOptionName        = "human"
)

var ObjectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated commands to interact with dag-pb objects. Use 'dag' or 'files' instead.",
		ShortDescription: `
'ipfs object' is a legacy plumbing command used to manipulate dag-pb objects
directly. Deprecated, use more modern 'ipfs dag' and 'ipfs files' instead.`,
	},

	Subcommands: map[string]*cmds.Command{
		"data":  ObjectDataCmd,
		"diff":  ObjectDiffCmd,
		"get":   ObjectGetCmd,
		"links": ObjectLinksCmd,
		"new":   ObjectNewCmd,
		"patch": ObjectPatchCmd,
		"put":   ObjectPutCmd,
		"stat":  ObjectStatCmd,
	},
}

// ObjectDataCmd object data command
var ObjectDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to read the raw bytes of a dag-pb object: use 'dag get' instead.",
		ShortDescription: `
'ipfs object data' is a deprecated plumbing command for retrieving the raw
bytes stored in a dag-pb node. It outputs to stdout, and <key> is a base58
encoded multihash. Provided for legacy reasons. Use 'ipfs dag get' instead.
`,
		LongDescription: `
'ipfs object data' is a deprecated plumbing command for retrieving the raw
bytes stored in a dag-pb node. It outputs to stdout, and <key> is a base58
encoded multihash. Provided for legacy reasons. Use 'ipfs dag get' instead.

Note that the "--encoding" option does not affect the output, since the output
is the raw data of the object.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		path := path.New(req.Arguments[0])

		data, err := api.Object().Data(req.Context, path)
		if err != nil {
			return err
		}

		return res.Emit(data)
	},
}

// ObjectLinksCmd object links command
var ObjectLinksCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to output links in the specified dag-pb object: use 'dag get' instead.",
		ShortDescription: `
'ipfs object links' is a plumbing command for retrieving the links from
a dag-pb node. It outputs to stdout, and <key> is a base58 encoded
multihash. Provided for legacy reasons. Use 'ipfs dag get' instead.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the dag-pb object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(headersOptionName, "v", "Print table headers (Hash, Size, Name)."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		path := path.New(req.Arguments[0])

		rp, err := api.ResolvePath(req.Context, path)
		if err != nil {
			return err
		}

		links, err := api.Object().Links(req.Context, rp)
		if err != nil {
			return err
		}

		outLinks := make([]Link, len(links))
		for i, link := range links {
			outLinks[i] = Link{
				Hash: enc.Encode(link.Cid),
				Name: link.Name,
				Size: link.Size,
			}
		}

		out := &Object{
			Hash:  enc.Encode(rp.Cid()),
			Links: outLinks,
		}

		return cmds.EmitOnce(res, out)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			tw := tabwriter.NewWriter(w, 1, 2, 1, ' ', 0)
			headers, _ := req.Options[headersOptionName].(bool)
			if headers {
				fmt.Fprintln(tw, "Hash\tSize\tName")
			}
			for _, link := range out.Links {
				fmt.Fprintf(tw, "%s\t%v\t%s\n", link.Hash, link.Size, cmdenv.EscNonPrint(link.Name))
			}
			tw.Flush()

			return nil
		}),
	},
	Type: &Object{},
}

// ObjectGetCmd object get command
var ObjectGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to get and serialize the dag-pb node. Use 'dag get' instead",
		ShortDescription: `
'ipfs object get' is a plumbing command for retrieving dag-pb nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.

DEPRECATED and provided for legacy reasons. Use 'ipfs dag get' instead.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the dag-pb object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(encodingOptionName, "Encoding type of the data field, either \"text\" or \"base64\".").WithDefault("text"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		path := path.New(req.Arguments[0])

		datafieldenc, _ := req.Options[encodingOptionName].(string)
		if err != nil {
			return err
		}

		nd, err := api.Object().Get(req.Context, path)
		if err != nil {
			return err
		}

		r, err := api.Object().Data(req.Context, path)
		if err != nil {
			return err
		}

		data, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		out, err := encodeData(data, datafieldenc)
		if err != nil {
			return err
		}

		node := &Node{
			Links: make([]Link, len(nd.Links())),
			Data:  out,
		}

		for i, link := range nd.Links() {
			node.Links[i] = Link{
				Hash: enc.Encode(link.Cid),
				Name: link.Name,
				Size: link.Size,
			}
		}

		return cmds.EmitOnce(res, node)
	},
	Type: Node{},
	Encoders: cmds.EncoderMap{
		cmds.Protobuf: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Node) error {
			// deserialize the Data field as text as this was the standard behaviour
			object, err := deserializeNode(out, "text")
			if err != nil {
				return nil
			}

			marshaled, err := object.Marshal()
			if err != nil {
				return err
			}
			_, err = w.Write(marshaled)
			return err
		}),
	},
}

// ObjectStatCmd object stat command
var ObjectStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to read stats for the dag-pb node. Use 'files stat' instead.",
		ShortDescription: `
'ipfs object stat' is a plumbing command to print dag-pb node statistics.
<key> is a base58 encoded multihash.

DEPRECATED: modern replacements are 'files stat' and 'dag stat'
`,
		LongDescription: `
'ipfs object stat' is a plumbing command to print dag-pb node statistics.
<key> is a base58 encoded multihash. It outputs to stdout:

	NumLinks        int number of links in link table
	BlockSize       int size of the raw, encoded data
	LinksSize       int size of the links segment
	DataSize        int size of the data segment
	CumulativeSize  int cumulative size of object and its references

DEPRECATED: Provided for legacy reasons. Modern replacements:

  For unixfs, 'ipfs files stat' can be used:

    $ ipfs files stat --with-local /ipfs/QmWfVY9y3xjsixTgbd9AorQxH7VtMpzfx2HaWtsoUYecaX
	QmWfVY9y3xjsixTgbd9AorQxH7VtMpzfx2HaWtsoUYecaX
	Size: 5
	CumulativeSize: 13
	ChildBlocks: 0
	Type: file
	Local: 13 B of 13 B (100.00%)

  Reported sizes are based on metadata present in root block, and should not be
  trusted.  A slower, but more secure alternative is 'ipfs dag stat', which
  will work for every DAG type.  It comes with a benefit of calculating the
  size by walking the DAG:

	$ ipfs dag stat /ipfs/QmWfVY9y3xjsixTgbd9AorQxH7VtMpzfx2HaWtsoUYecaX
	Size: 13, NumBlocks: 1
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(humanOptionName, "Print sizes in human readable format (e.g., 1K 234M 2G)"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		ns, err := api.Object().Stat(req.Context, path.New(req.Arguments[0]))
		if err != nil {
			return err
		}

		oldStat := &ipld.NodeStat{
			Hash:           enc.Encode(ns.Cid),
			NumLinks:       ns.NumLinks,
			BlockSize:      ns.BlockSize,
			LinksSize:      ns.LinksSize,
			DataSize:       ns.DataSize,
			CumulativeSize: ns.CumulativeSize,
		}

		return cmds.EmitOnce(res, oldStat)
	},
	Type: ipld.NodeStat{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *ipld.NodeStat) error {
			wtr := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
			defer wtr.Flush()
			fw := func(s string, n int) {
				fmt.Fprintf(wtr, "%s:\t%d\n", s, n)
			}
			human, _ := req.Options[humanOptionName].(bool)
			fw("NumLinks", out.NumLinks)
			fw("BlockSize", out.BlockSize)
			fw("LinksSize", out.LinksSize)
			fw("DataSize", out.DataSize)
			if human {
				fmt.Fprintf(wtr, "%s:\t%s\n", "CumulativeSize", humanize.Bytes(uint64(out.CumulativeSize)))
			} else {
				fw("CumulativeSize", out.CumulativeSize)
			}

			return nil
		}),
	},
}

// ObjectPutCmd object put command
var ObjectPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to store input as a DAG object. Use 'dag put' instead.",
		ShortDescription: `
'ipfs object put' is a plumbing command for storing dag-pb nodes.
It reads from stdin, and the output is a base58 encoded multihash.

DEPRECATED and provided for legacy reasons. Use 'ipfs dag put' instead.
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a dag-pb object.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(inputencOptionName, "Encoding type of input data. One of: {\"protobuf\", \"json\"}.").WithDefault("json"),
		cmds.StringOption(datafieldencOptionName, "Encoding type of the data field, either \"text\" or \"base64\".").WithDefault("text"),
		cmds.BoolOption(pinOptionName, "Pin this object when adding."),
		cmds.BoolOption(quietOptionName, "q", "Write minimal output."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}

		inputenc, _ := req.Options[inputencOptionName].(string)
		if err != nil {
			return err
		}

		datafieldenc, _ := req.Options[datafieldencOptionName].(string)
		if err != nil {
			return err
		}

		dopin, _ := req.Options[pinOptionName].(bool)
		if err != nil {
			return err
		}

		p, err := api.Object().Put(req.Context, file,
			options.Object.DataType(datafieldenc),
			options.Object.InputEnc(inputenc),
			options.Object.Pin(dopin))
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: enc.Encode(p.Cid())})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			quiet, _ := req.Options[quietOptionName].(bool)

			o := out.Hash
			if !quiet {
				o = "added " + o
			}

			fmt.Fprintln(w, o)

			return nil
		}),
	},
	Type: Object{},
}

// ObjectNewCmd object new command
var ObjectNewCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Deprecated way to create a new dag-pb object from a template.",
		ShortDescription: `
'ipfs object new' is a plumbing command for creating new dag-pb nodes.
DEPRECATED and provided for legacy reasons. Use 'dag put' and 'files' instead.
`,
		LongDescription: `
'ipfs object new' is a plumbing command for creating new dag-pb nodes.
By default it creates and returns a new empty merkledag node, but
you may pass an optional template argument to create a preformatted
node.

Available templates:
	* unixfs-dir

DEPRECATED and provided for legacy reasons. Use 'dag put' and 'files' instead.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("template", false, false, "Template to use. Optional."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		enc, err := cmdenv.GetLowLevelCidEncoder(req)
		if err != nil {
			return err
		}

		template := "empty"
		if len(req.Arguments) == 1 {
			template = req.Arguments[0]
		}

		nd, err := api.Object().New(req.Context, options.Object.Type(template))
		if err != nil && err != io.EOF {
			return err
		}

		return cmds.EmitOnce(res, &Object{Hash: enc.Encode(nd.Cid())})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *Object) error {
			fmt.Fprintln(w, out.Hash)
			return nil
		}),
	},
	Type: Object{},
}

// converts the Node object into a real dag.ProtoNode
func deserializeNode(nd *Node, dataFieldEncoding string) (*dag.ProtoNode, error) {
	dagnode := new(dag.ProtoNode)
	switch dataFieldEncoding {
	case "text":
		dagnode.SetData([]byte(nd.Data))
	case "base64":
		data, err := base64.StdEncoding.DecodeString(nd.Data)
		if err != nil {
			return nil, err
		}
		dagnode.SetData(data)
	default:
		return nil, ErrDataEncoding
	}

	links := make([]*ipld.Link, len(nd.Links))
	for i, link := range nd.Links {
		c, err := cid.Decode(link.Hash)
		if err != nil {
			return nil, err
		}
		links[i] = &ipld.Link{
			Name: link.Name,
			Size: link.Size,
			Cid:  c,
		}
	}
	dagnode.SetLinks(links)

	return dagnode, nil
}

func encodeData(data []byte, encoding string) (string, error) {
	switch encoding {
	case "text":
		return string(data), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(data), nil
	}

	return "", ErrDataEncoding
}
