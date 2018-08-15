package objectcmd

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/tabwriter"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	lgc "github.com/ipfs/go-ipfs/commands/legacy"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	coreiface "github.com/ipfs/go-ipfs/core/coreapi/interface"
	"github.com/ipfs/go-ipfs/core/coreapi/interface/options"

	cmdkit "gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	dag "gx/ipfs/QmQzSpSjkdGHW6WFBhUG6P3t9K8yv7iucucT1cQaqJ6tgd/go-merkledag"
	cmds "gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"
	cid "gx/ipfs/QmYjnkEL7i731PirfVH1sis89evN7jt4otSHw5D2xXXwUV/go-cid"
	ipld "gx/ipfs/QmaA8GkXUYinkkndvg7T6Tx7gYXemhxjaxLisEPes7Rf1P/go-ipld-format"
)

// ErrObjectTooLarge is returned when too much data was read from stdin. current limit 2m
var ErrObjectTooLarge = errors.New("input object was too large. limit is 2mbytes")

const inputLimit = 2 << 20

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

var ObjectCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Interact with IPFS objects.",
		ShortDescription: `
'ipfs object' is a plumbing command used to manipulate DAG objects
directly.`,
	},

	Subcommands: map[string]*cmds.Command{
		"data":  lgc.NewCommand(ObjectDataCmd),
		"diff":  lgc.NewCommand(ObjectDiffCmd),
		"get":   lgc.NewCommand(ObjectGetCmd),
		"links": lgc.NewCommand(ObjectLinksCmd),
		"new":   lgc.NewCommand(ObjectNewCmd),
		"patch": ObjectPatchCmd,
		"put":   lgc.NewCommand(ObjectPutCmd),
		"stat":  lgc.NewCommand(ObjectStatCmd),
	},
}

var ObjectDataCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
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

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := api.Object().Data(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(data)
	},
}

var ObjectLinksCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Output the links pointed to by the specified object.",
		ShortDescription: `
'ipfs object links' is a plumbing command for retrieving the links from
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("headers", "v", "Print table headers (Hash, Size, Name)."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option("headers").Bool(); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		rp, err := api.ResolvePath(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		links, err := api.Object().Links(req.Context(), rp)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outLinks := make([]Link, len(links))
		for i, link := range links {
			outLinks[i] = Link{
				Hash: link.Cid.String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		out := Object{
			Hash:  rp.Cid().String(),
			Links: outLinks,
		}

		res.SetOutput(out)
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			object, ok := v.(*Object)
			if !ok {
				return nil, e.TypeErr(object, v)
			}

			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			headers, _, _ := res.Request().Option("headers").Bool()
			if headers {
				fmt.Fprintln(w, "Hash\tSize\tName\t")
			}
			for _, link := range object.Links {
				fmt.Fprintf(w, "%s\t%v\t%s\t\n", link.Hash, link.Size, link.Name)
			}
			w.Flush()
			return buf, nil
		},
	},
	Type: Object{},
}

var ObjectGetCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
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

The encoding of the object's data field can be specifed by using the
--data-encoding flag

Supported values are:
	* "text" (default)
	* "base64"
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("data-encoding", "Encoding type of the data field, either \"text\" or \"base64\".").WithDefault("text"),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		datafieldenc, _, err := req.Option("data-encoding").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		nd, err := api.Object().Get(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		r, err := api.Object().Data(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data, err := ioutil.ReadAll(r)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		out, err := encodeData(data, datafieldenc)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		node := &Node{
			Links: make([]Link, len(nd.Links())),
			Data:  out,
		}

		for i, link := range nd.Links() {
			node.Links[i] = Link{
				Hash: link.Cid.String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		res.SetOutput(node)
	},
	Type: Node{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Protobuf: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			node, ok := v.(*Node)
			if !ok {
				return nil, e.TypeErr(node, v)
			}

			// deserialize the Data field as text as this was the standard behaviour
			object, err := deserializeNode(node, "text")
			if err != nil {
				return nil, err
			}

			marshaled, err := object.Marshal()
			if err != nil {
				return nil, err
			}
			return bytes.NewReader(marshaled), nil
		},
	},
}

var ObjectStatCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
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

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		path, err := coreiface.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		ns, err := api.Object().Stat(req.Context(), path)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		oldStat := &ipld.NodeStat{
			Hash:           ns.Cid.String(),
			NumLinks:       ns.NumLinks,
			BlockSize:      ns.BlockSize,
			LinksSize:      ns.LinksSize,
			DataSize:       ns.DataSize,
			CumulativeSize: ns.CumulativeSize,
		}

		res.SetOutput(oldStat)
	},
	Type: ipld.NodeStat{},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			ns, ok := v.(*ipld.NodeStat)
			if !ok {
				return nil, e.TypeErr(ns, v)
			}

			buf := new(bytes.Buffer)
			w := func(s string, n int) {
				fmt.Fprintf(buf, "%s: %d\n", s, n)
			}
			w("NumLinks", ns.NumLinks)
			w("BlockSize", ns.BlockSize)
			w("LinksSize", ns.LinksSize)
			w("DataSize", ns.DataSize)
			w("CumulativeSize", ns.CumulativeSize)

			return buf, nil
		},
	},
}

var ObjectPutCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
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

	Arguments: []cmdkit.Argument{
		cmdkit.FileArg("data", true, false, "Data to be stored as a DAG object.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption("inputenc", "Encoding type of input data. One of: {\"protobuf\", \"json\"}.").WithDefault("json"),
		cmdkit.StringOption("datafieldenc", "Encoding type of the data field, either \"text\" or \"base64\".").WithDefault("text"),
		cmdkit.BoolOption("pin", "Pin this object when adding."),
		cmdkit.BoolOption("quiet", "q", "Write minimal output."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil && err != io.EOF {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		inputenc, _, err := req.Option("inputenc").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		datafieldenc, _, err := req.Option("datafieldenc").String()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dopin, _, err := req.Option("pin").Bool()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		p, err := api.Object().Put(req.Context(), input,
			options.Object.DataType(datafieldenc),
			options.Object.InputEnc(inputenc),
			options.Object.Pin(dopin))
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: p.Cid().String()})
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			quiet, _, _ := res.Request().Option("quiet").Bool()

			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}
			obj, ok := v.(*Object)
			if !ok {
				return nil, e.TypeErr(obj, v)
			}

			out := obj.Hash + "\n"
			if !quiet {
				out = "added " + out
			}

			return strings.NewReader(out), nil
		},
	},
	Type: Object{},
}

var ObjectNewCmd = &oldcmds.Command{
	Helptext: cmdkit.HelpText{
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
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("template", false, false, "Template to use. Optional."),
	},
	Run: func(req oldcmds.Request, res oldcmds.Response) {
		api, err := req.InvocContext().GetApi()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		template := "empty"
		if len(req.Arguments()) == 1 {
			template = req.Arguments()[0]
		}

		nd, err := api.Object().New(req.Context(), options.Object.Type(template))
		if err != nil && err != io.EOF {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(&Object{Hash: nd.Cid().String()})
	},
	Marshalers: oldcmds.MarshalerMap{
		oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			obj, ok := v.(*Object)
			if !ok {
				return nil, e.TypeErr(obj, v)
			}

			return strings.NewReader(obj.Hash + "\n"), nil
		},
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
		return nil, fmt.Errorf("unkown data field encoding")
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

// copy+pasted from ../commands.go
func unwrapOutput(i interface{}) (interface{}, error) {
	var (
		ch <-chan interface{}
		ok bool
	)

	if ch, ok = i.(<-chan interface{}); !ok {
		return nil, e.TypeErr(ch, i)
	}

	return <-ch, nil
}

func encodeData(data []byte, encoding string) (string, error) {
	switch encoding {
	case "text":
		return string(data), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(data), nil
	}

	return "", fmt.Errorf("unkown data field encoding")
}
