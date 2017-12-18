package objectcmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	ft "github.com/ipfs/go-ipfs/unixfs"

	node "gx/ipfs/QmNwUEK7QbwSqyKBu3mMtToo8SUc6wQJ7gdZq4gGGJqfnf/go-ipld-format"
	cmdkit "gx/ipfs/QmSRaAPPNxyhnXeDa5NXtZ2CWBYJ6BRWNQp6gKxhPcoqDM/go-ipfs-cmdkit"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
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

var ObjectDataCmd = &cmds.Command{
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fpath, err := path.ParsePath(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		node, err := core.Resolve(req.Context(), n.Namesys, n.Resolver, fpath)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pbnode, ok := node.(*dag.ProtoNode)
		if !ok {
			res.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(bytes.NewReader(pbnode.Data()))
	},
}

var ObjectLinksCmd = &cmds.Command{
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// get options early -> exit early in case of error
		if _, _, err := req.Option("headers").Bool(); err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])
		node, err := core.Resolve(req.Context(), n.Namesys, n.Resolver, fpath)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		output, err := getOutput(node)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
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

var ObjectGetCmd = &cmds.Command{
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
(Specified by the "--encoding" or "--enc" flag)`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format.").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := core.Resolve(req.Context(), n.Namesys, n.Resolver, fpath)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pbo, ok := object.(*dag.ProtoNode)
		if !ok {
			res.SetError(dag.ErrNotProtobuf, cmdkit.ErrNormal)
			return
		}

		node := &Node{
			Links: make([]Link, len(object.Links())),
			Data:  string(pbo.Data()),
		}

		for i, link := range object.Links() {
			node.Links[i] = Link{
				Hash: link.Cid.String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		res.SetOutput(node)
	},
	Type: Node{},
	Marshalers: cmds.MarshalerMap{
		cmds.Protobuf: func(res cmds.Response) (io.Reader, error) {
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

var ObjectStatCmd = &cmds.Command{
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := core.Resolve(req.Context(), n.Namesys, n.Resolver, fpath)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		ns, err := object.Stat()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		res.SetOutput(ns)
	},
	Type: node.NodeStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			ns, ok := v.(*node.NodeStat)
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

var ObjectPutCmd = &cmds.Command{
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
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

		if dopin {
			defer n.Blockstore.PinLock().Unlock()
		}

		objectCid, err := objectPut(n, input, inputenc, datafieldenc)
		if err != nil {
			errType := cmdkit.ErrNormal
			if err == ErrUnknownObjectEnc {
				errType = cmdkit.ErrClient
			}
			res.SetError(err, errType)
			return
		}

		if dopin {
			n.Pinning.PinWithMode(objectCid, pin.Recursive)
			err = n.Pinning.Flush()
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		res.SetOutput(&Object{Hash: objectCid.String()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
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

var ObjectNewCmd = &cmds.Command{
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
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		node := new(dag.ProtoNode)
		if len(req.Arguments()) == 1 {
			template := req.Arguments()[0]
			var err error
			node, err = nodeFromTemplate(template)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		k, err := n.DAG.Add(node)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		res.SetOutput(&Object{Hash: k.String()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
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

func nodeFromTemplate(template string) (*dag.ProtoNode, error) {
	switch template {
	case "unixfs-dir":
		return ft.EmptyDirNode(), nil
	default:
		return nil, fmt.Errorf("template '%s' not found", template)
	}
}

// ErrEmptyNode is returned when the input to 'ipfs object put' contains no data
var ErrEmptyNode = errors.New("no data or links in this node")

// objectPut takes a format option, serializes bytes from stdin and updates the dag with that data
func objectPut(n *core.IpfsNode, input io.Reader, encoding string, dataFieldEncoding string) (*cid.Cid, error) {

	data, err := ioutil.ReadAll(io.LimitReader(input, inputLimit+10))
	if err != nil {
		return nil, err
	}

	if len(data) >= inputLimit {
		return nil, ErrObjectTooLarge
	}

	var dagnode *dag.ProtoNode
	switch getObjectEnc(encoding) {
	case objectEncodingJSON:
		node := new(Node)
		err = json.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		// check that we have data in the Node to add
		// otherwise we will add the empty object without raising an error
		if NodeEmpty(node) {
			return nil, ErrEmptyNode
		}

		dagnode, err = deserializeNode(node, dataFieldEncoding)
		if err != nil {
			return nil, err
		}

	case objectEncodingProtobuf:
		dagnode, err = dag.DecodeProtobuf(data)

	case objectEncodingXML:
		node := new(Node)
		err = xml.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		// check that we have data in the Node to add
		// otherwise we will add the empty object without raising an error
		if NodeEmpty(node) {
			return nil, ErrEmptyNode
		}

		dagnode, err = deserializeNode(node, dataFieldEncoding)
		if err != nil {
			return nil, err
		}

	default:
		return nil, ErrUnknownObjectEnc
	}

	if err != nil {
		return nil, err
	}

	_, err = n.DAG.Add(dagnode)
	if err != nil {
		return nil, err
	}

	return dagnode.Cid(), nil
}

// ErrUnknownObjectEnc is returned if a invalid encoding is supplied
var ErrUnknownObjectEnc = errors.New("unknown object encoding")

type objectEncoding string

const (
	objectEncodingJSON     objectEncoding = "json"
	objectEncodingProtobuf                = "protobuf"
	objectEncodingXML                     = "xml"
)

func getObjectEnc(o interface{}) objectEncoding {
	v, ok := o.(string)
	if !ok {
		// chosen as default because it's human readable
		return objectEncodingJSON
	}

	return objectEncoding(v)
}

func getOutput(dagnode node.Node) (*Object, error) {
	c := dagnode.Cid()
	output := &Object{
		Hash:  c.String(),
		Links: make([]Link, len(dagnode.Links())),
	}

	for i, link := range dagnode.Links() {
		output.Links[i] = Link{
			Name: link.Name,
			Hash: link.Cid.String(),
			Size: link.Size,
		}
	}

	return output, nil
}

// converts the Node object into a real dag.ProtoNode
func deserializeNode(nd *Node, dataFieldEncoding string) (*dag.ProtoNode, error) {
	dagnode := new(dag.ProtoNode)
	switch dataFieldEncoding {
	case "text":
		dagnode.SetData([]byte(nd.Data))
	case "base64":
		data, _ := base64.StdEncoding.DecodeString(nd.Data)
		dagnode.SetData(data)
	default:
		return nil, fmt.Errorf("Unkown data field encoding")
	}

	dagnode.SetLinks(make([]*node.Link, len(nd.Links)))
	for i, link := range nd.Links {
		c, err := cid.Decode(link.Hash)
		if err != nil {
			return nil, err
		}
		dagnode.Links()[i] = &node.Link{
			Name: link.Name,
			Size: link.Size,
			Cid:  c,
		}
	}

	return dagnode, nil
}

func NodeEmpty(node *Node) bool {
	return (node.Data == "" && len(node.Links) == 0)
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
