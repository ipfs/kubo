package objectcmd

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"text/tabwriter"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"

	humanize "github.com/dustin/go-humanize"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs-cmds"
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
		Tagline: "Interact with IPFS objects.",
		ShortDescription: `
'ipfs object' is a plumbing command used to manipulate DAG objects
directly.`,
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
		Tagline: "Output the raw bytes of an IPFS object.",
		ShortDescription: `
'ipfs object data' is a plumbing command for retrieving the raw bytes stored
in a DAG node. It outputs to stdout, and <key> is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs object data' is a plumbing command for retrieving the raw bytes stored
in a DAG node. It outputs to stdout, and <key> is a base58 encoded multihash.

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
		Tagline: "Output the links pointed to by the specified object.",
		ShortDescription: `
'ipfs object links' is a plumbing command for retrieving the links from
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
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
		Tagline: "Get and serialize the DAG node named by <key>.",
		ShortDescription: `
'ipfs object get' is a plumbing command for retrieving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs object get' is a plumbing command for retrieving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.

This command outputs data in the following encodings:
  * "protobuf"
  * "json"
  * "xml"
(Specified by the "--encoding" or "--enc" flag)

The encoding of the object's data field can be specified by using the
--data-encoding flag

Supported values are:
	* "text" (default)
	* "base64"
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
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
		Tagline: "Get stats for the DAG node named by <key>.",
		ShortDescription: `
'ipfs object stat' is a plumbing command to print DAG node statistics.
<key> is a base58 encoded multihash. It outputs to stdout:

	NumLinks        int number of links in link table
	BlockSize       int size of the raw, encoded data
	LinksSize       int size of the links segment
	DataSize        int size of the data segment
	CumulativeSize  int cumulative size of object and its references
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
		Tagline: "Store input as a DAG object, print its key.",
		ShortDescription: `
'ipfs object put' is a plumbing command for storing DAG nodes.
It reads from stdin, and the output is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs object put' is a plumbing command for storing DAG nodes.
It reads from stdin, and the output is a base58 encoded multihash.

Data should be in the format specified by the --inputenc flag.
--inputenc may be one of the following:
	* "protobuf"
	* "json" (default)

Examples:

	$ echo '{ "Data": "abc" }' | ipfs object put

This creates a node with the data 'abc' and no links. For an object with
links, create a file named 'node.json' with the contents:

    {
        "Data": "another",
        "Links": [ {
            "Name": "some link",
            "Hash": "QmXg9Pp2ytZ14xgmQjYEiHjVjMFXzCVVEcRTWJBmLgR39V",
            "Size": 8
        } ]
    }

And then run:

	$ ipfs object put node.json
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a DAG object.").EnableStdin(),
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
		Tagline: "Create a new object from an ipfs template.",
		ShortDescription: `
'ipfs object new' is a plumbing command for creating new DAG nodes.
`,
		LongDescription: `
'ipfs object new' is a plumbing command for creating new DAG nodes.
By default it creates and returns a new empty merkledag node, but
you may pass an optional template argument to create a preformatted
node.

Available templates:
	* unixfs-dir
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
