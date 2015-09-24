package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"text/tabwriter"

	mh "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	key "github.com/ipfs/go-ipfs/blocks/key"
	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	dag "github.com/ipfs/go-ipfs/merkledag"
	dagutils "github.com/ipfs/go-ipfs/merkledag/utils"
	path "github.com/ipfs/go-ipfs/path"
	ft "github.com/ipfs/go-ipfs/unixfs"
	u "github.com/ipfs/go-ipfs/util"
)

// ErrObjectTooLarge is returned when too much data was read from stdin. current limit 512k
var ErrObjectTooLarge = errors.New("input object was too large. limit is 512kbytes")

const inputLimit = 512 * 1024

type Node struct {
	Links []Link
	Data  string
}

type Link struct {
	Name, Hash string
	Size       uint64
}

type Object struct {
	Hash  string
	Links []Link
}

var ObjectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipfs objects",
		ShortDescription: `
'ipfs object' is a plumbing command used to manipulate DAG objects
directly.`,
		Synopsis: `
ipfs object get <key>       - Get the DAG node named by <key>
ipfs object put <data>      - Stores input, outputs its key
ipfs object data <key>      - Outputs raw bytes in an object
ipfs object links <key>     - Outputs links pointed to by object
ipfs object stat <key>      - Outputs statistics of object
ipfs object new <template>  - Create new ipfs objects
ipfs object patch <args>    - Create new object from old ones
`,
	},

	Subcommands: map[string]*cmds.Command{
		"data":  objectDataCmd,
		"links": objectLinksCmd,
		"get":   objectGetCmd,
		"put":   objectPutCmd,
		"stat":  objectStatCmd,
		"new":   objectNewCmd,
		"patch": objectPatchCmd,
	},
}

var objectDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs the raw bytes in an IPFS object",
		ShortDescription: `
ipfs object data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
		LongDescription: `
ipfs object data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.

Note that the "--encoding" option does not affect the output, since the
output is the raw data of the object.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])
		node, err := core.Resolve(req.Context(), n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(bytes.NewReader(node.Data))
	},
}

var objectLinksCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs the links pointed to by the specified object",
		ShortDescription: `
'ipfs object links' is a plumbing command for retreiving the links from
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])
		node, err := core.Resolve(req.Context(), n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		output, err := getOutput(node)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*Object)
			buf := new(bytes.Buffer)
			w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
			fmt.Fprintln(w, "Hash\tSize\tName\t")
			for _, link := range object.Links {
				fmt.Fprintf(w, "%s\t%v\t%s\t\n", link.Hash, link.Size, link.Name)
			}
			w.Flush()
			return buf, nil
		},
	},
	Type: Object{},
}

var objectGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get and serialize the DAG node named by <key>",
		ShortDescription: `
'ipfs object get' is a plumbing command for retreiving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.
`,
		LongDescription: `
'ipfs object get' is a plumbing command for retreiving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding"
flag. It outputs to stdout, and <key> is a base58 encoded multihash.

This command outputs data in the following encodings:
  * "protobuf"
  * "json"
  * "xml"
(Specified by the "--encoding" or "-enc" flag)`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve (in base58-encoded multihash format)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := core.Resolve(req.Context(), n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		node := &Node{
			Links: make([]Link, len(object.Links)),
			Data:  string(object.Data),
		}

		for i, link := range object.Links {
			node.Links[i] = Link{
				Hash: link.Hash.B58String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		res.SetOutput(node)
	},
	Type: Node{},
	Marshalers: cmds.MarshalerMap{
		cmds.EncodingType("protobuf"): func(res cmds.Response) (io.Reader, error) {
			node := res.Output().(*Node)
			object, err := deserializeNode(node)
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

var objectStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get stats for the DAG node named by <key>",
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
		cmds.StringArg("key", true, false, "Key of the object to retrieve (in base58-encoded multihash format)").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fpath := path.Path(req.Arguments()[0])

		object, err := core.Resolve(req.Context(), n, fpath)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ns, err := object.Stat()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(ns)
	},
	Type: dag.NodeStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ns := res.Output().(*dag.NodeStat)

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

var objectPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Stores input as a DAG object, outputs its key",
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

	echo '{ "Data": "abc" }' | ipfs object put

This creates a node with the data "abc" and no links. For an object with links,
create a file named node.json with the contents:

    {
        "Data": "another",
        "Links": [ {
            "Name": "some link",
            "Hash": "QmXg9Pp2ytZ14xgmQjYEiHjVjMFXzCVVEcRTWJBmLgR39V",
            "Size": 8
        } ]
    }

and then run

	ipfs object put node.json
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a DAG object").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("inputenc", "Encoding type of input data, either \"protobuf\" or \"json\""),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		input, err := req.Files().NextFile()
		if err != nil && err != io.EOF {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		inputenc, found, err := req.Option("inputenc").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			inputenc = "json"
		}

		output, err := objectPut(n, input, inputenc)
		if err != nil {
			errType := cmds.ErrNormal
			if err == ErrUnknownObjectEnc {
				errType = cmds.ErrClient
			}
			res.SetError(err, errType)
			return
		}

		res.SetOutput(output)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*Object)
			return strings.NewReader("added " + object.Hash), nil
		},
	},
	Type: Object{},
}

var objectNewCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "creates a new object from an ipfs template",
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
		cmds.StringArg("template", false, false, "optional template to use"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		node := new(dag.Node)
		if len(req.Arguments()) == 1 {
			template := req.Arguments()[0]
			var err error
			node, err = nodeFromTemplate(template)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		k, err := n.DAG.Add(node)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		res.SetOutput(&Object{Hash: k.B58String()})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*Object)
			return strings.NewReader(object.Hash + "\n"), nil
		},
	},
	Type: Object{},
}

var objectPatchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new merkledag object based on an existing one",
		ShortDescription: `
'ipfs object patch <root> <cmd> <args>' is a plumbing command used to
build custom DAG objects. It adds and removes links from objects, creating a new
object as a result. This is the merkle-dag version of modifying an object. It
can also set the data inside a node with 'set-data' and append to that data as
well with 'append-data'.

Patch commands:
    add-link <name> <ref>     - adds a link to a node
    rm-link <name>            - removes a link from a node
    set-data                  - sets a nodes data from stdin
    append-data               - appends to a nodes data from stdin

Examples:

    EMPTY_DIR=$(ipfs object new unixfs-dir)
    BAR=$(echo "bar" | ipfs add -q)
    ipfs object patch $EMPTY_DIR add-link foo $BAR

This takes an empty directory, and adds a link named foo under it, pointing to
a file containing 'bar', and returns the hash of the new object.

    ipfs object patch $FOO_BAR rm-link foo

This removes the link named foo from the hash in $FOO_BAR and returns the
resulting object hash.

The data inside the node can be modified as well:

    ipfs object patch $FOO_BAR set-data < file.dat
    ipfs object patch $FOO_BAR append-data < file.dat

`,
	},
	Options: []cmds.Option{
		cmds.BoolOption("create", "p", "create intermediate directories on add-link"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("root", true, false, "the hash of the node to modify"),
		cmds.StringArg("command", true, false, "the operation to perform"),
		cmds.StringArg("args", true, true, "extra arguments").EnableStdin(),
	},
	Type: Object{},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		rootarg := req.Arguments()[0]
		if strings.HasPrefix(rootarg, "/ipfs/") {
			rootarg = rootarg[6:]
		}
		rhash := key.B58KeyDecode(rootarg)
		if rhash == "" {
			res.SetError(fmt.Errorf("incorrectly formatted root hash: %s", req.Arguments()[0]), cmds.ErrNormal)
			return
		}

		rnode, err := nd.DAG.Get(req.Context(), rhash)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		action := req.Arguments()[1]

		switch action {
		case "add-link":
			k, err := addLinkCaller(req, rnode)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&Object{Hash: k.B58String()})
		case "rm-link":
			k, err := rmLinkCaller(req, rnode)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&Object{Hash: k.B58String()})
		case "set-data":
			k, err := setDataCaller(req, rnode)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&Object{Hash: k.B58String()})
		case "append-data":
			k, err := appendDataCaller(req, rnode)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&Object{Hash: k.B58String()})
		default:
			res.SetError(fmt.Errorf("unrecognized subcommand"), cmds.ErrNormal)
			return
		}
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			o, ok := res.Output().(*Object)
			if !ok {
				return nil, u.ErrCast()
			}

			return strings.NewReader(o.Hash + "\n"), nil
		},
	},
}

func appendDataCaller(req cmds.Request, root *dag.Node) (key.Key, error) {
	if len(req.Arguments()) < 3 {
		return "", fmt.Errorf("not enough arguments for set-data")
	}

	nd, err := req.InvocContext().GetNode()
	if err != nil {
		return "", err
	}

	root.Data = append(root.Data, []byte(req.Arguments()[2])...)

	newkey, err := nd.DAG.Add(root)
	if err != nil {
		return "", err
	}

	return newkey, nil
}

func setDataCaller(req cmds.Request, root *dag.Node) (key.Key, error) {
	if len(req.Arguments()) < 3 {
		return "", fmt.Errorf("not enough arguments for set-data")
	}

	nd, err := req.InvocContext().GetNode()
	if err != nil {
		return "", err
	}

	root.Data = []byte(req.Arguments()[2])

	newkey, err := nd.DAG.Add(root)
	if err != nil {
		return "", err
	}

	return newkey, nil
}

func rmLinkCaller(req cmds.Request, root *dag.Node) (key.Key, error) {
	if len(req.Arguments()) < 3 {
		return "", fmt.Errorf("not enough arguments for rm-link")
	}

	nd, err := req.InvocContext().GetNode()
	if err != nil {
		return "", err
	}

	path := req.Arguments()[2]

	e := dagutils.NewDagEditor(nd.DAG, root)

	err = e.RmLink(req.Context(), path)
	if err != nil {
		return "", err
	}

	nnode := e.GetNode()

	return nnode.Key()
}

func addLinkCaller(req cmds.Request, root *dag.Node) (key.Key, error) {
	if len(req.Arguments()) < 4 {
		return "", fmt.Errorf("not enough arguments for add-link")
	}

	nd, err := req.InvocContext().GetNode()
	if err != nil {
		return "", err
	}

	path := req.Arguments()[2]
	childk := key.B58KeyDecode(req.Arguments()[3])

	create, _, err := req.Option("create").Bool()
	if err != nil {
		return "", err
	}

	var createfunc func() *dag.Node
	if create {
		createfunc = func() *dag.Node {
			return &dag.Node{Data: ft.FolderPBData()}
		}
	}

	e := dagutils.NewDagEditor(nd.DAG, root)

	childnd, err := nd.DAG.Get(req.Context(), childk)
	if err != nil {
		return "", err
	}

	err = e.InsertNodeAtPath(req.Context(), path, childnd, createfunc)
	if err != nil {
		return "", err
	}

	nnode := e.GetNode()

	return nnode.Key()
}

func nodeFromTemplate(template string) (*dag.Node, error) {
	switch template {
	case "unixfs-dir":
		nd := new(dag.Node)
		nd.Data = ft.FolderPBData()
		return nd, nil
	default:
		return nil, fmt.Errorf("template '%s' not found", template)
	}
}

// ErrEmptyNode is returned when the input to 'ipfs object put' contains no data
var ErrEmptyNode = errors.New("no data or links in this node")

// objectPut takes a format option, serializes bytes from stdin and updates the dag with that data
func objectPut(n *core.IpfsNode, input io.Reader, encoding string) (*Object, error) {

	data, err := ioutil.ReadAll(io.LimitReader(input, inputLimit+10))
	if err != nil {
		return nil, err
	}

	if len(data) >= inputLimit {
		return nil, ErrObjectTooLarge
	}

	var dagnode *dag.Node
	switch getObjectEnc(encoding) {
	case objectEncodingJSON:
		node := new(Node)
		err = json.Unmarshal(data, node)
		if err != nil {
			return nil, err
		}

		// check that we have data in the Node to add
		// otherwise we will add the empty object without raising an error
		if node.Data == "" && len(node.Links) == 0 {
			return nil, ErrEmptyNode
		}

		dagnode, err = deserializeNode(node)
		if err != nil {
			return nil, err
		}

	case objectEncodingProtobuf:
		dagnode, err = dag.Decoded(data)

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

	return getOutput(dagnode)
}

// ErrUnknownObjectEnc is returned if a invalid encoding is supplied
var ErrUnknownObjectEnc = errors.New("unknown object encoding")

type objectEncoding string

const (
	objectEncodingJSON     objectEncoding = "json"
	objectEncodingProtobuf                = "protobuf"
)

func getObjectEnc(o interface{}) objectEncoding {
	v, ok := o.(string)
	if !ok {
		// chosen as default because it's human readable
		log.Warning("option is not a string - falling back to json")
		return objectEncodingJSON
	}

	return objectEncoding(v)
}

func getOutput(dagnode *dag.Node) (*Object, error) {
	key, err := dagnode.Key()
	if err != nil {
		return nil, err
	}

	output := &Object{
		Hash:  key.Pretty(),
		Links: make([]Link, len(dagnode.Links)),
	}

	for i, link := range dagnode.Links {
		output.Links[i] = Link{
			Name: link.Name,
			Hash: link.Hash.B58String(),
			Size: link.Size,
		}
	}

	return output, nil
}

// converts the Node object into a real dag.Node
func deserializeNode(node *Node) (*dag.Node, error) {
	dagnode := new(dag.Node)
	dagnode.Data = []byte(node.Data)
	dagnode.Links = make([]*dag.Link, len(node.Links))
	for i, link := range node.Links {
		hash, err := mh.FromB58String(link.Hash)
		if err != nil {
			return nil, err
		}
		dagnode.Links[i] = &dag.Link{
			Name: link.Name,
			Size: link.Size,
			Hash: hash,
		}
	}

	return dagnode, nil
}
