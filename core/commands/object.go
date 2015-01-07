package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

// ErrObjectTooLarge is returned when too much data was read from stdin. current limit 512k
var ErrObjectTooLarge = errors.New("input object was too large. limit is 512kbytes")

const inputLimit = 512 * 1024

type Node struct {
	Links []Link
	Data  []byte
}

var ObjectCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with ipfs objects",
		ShortDescription: `
'ipfs object' is a plumbing command used to manipulate DAG objects
directly.`,
		Synopsis: `
ipfs object get <key>             - Get the DAG node named by <key>
ipfs object put <data> <encoding> - Stores input, outputs its key
ipfs object data <key>            - Outputs raw bytes in an object
ipfs object links <key>           - Outputs links pointed to by object
ipfs object stat <key>            - Outputs statistics of object
`,
	},

	Subcommands: map[string]*cmds.Command{
		"data":  objectDataCmd,
		"links": objectLinksCmd,
		"get":   objectGetCmd,
		"put":   objectPutCmd,
		"stat":  objectStatCmd,
	},
}

var objectDataCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Outputs the raw bytes in an IPFS object",
		ShortDescription: `
ipfs data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.
`,
		LongDescription: `
ipfs data is a plumbing command for retreiving the raw bytes stored in
a DAG node. It outputs to stdout, and <key> is a base58 encoded
multihash.

Note that the "--encoding" option does not affect the output, since the
output is the raw data of the object.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key := req.Arguments()[0]
		return objectData(n, key)
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
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key := req.Arguments()[0]
		return objectLinks(n, key)
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*Object)
			marshalled := marshalLinks(object.Links)
			return strings.NewReader(marshalled), nil
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
		cmds.StringArg("key", true, false, "Key of the object to retrieve (in base58-encoded multihash format)"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key := req.Arguments()[0]

		object, err := objectGet(n, key)
		if err != nil {
			return nil, err
		}

		node := &Node{
			Links: make([]Link, len(object.Links)),
			Data:  object.Data,
		}

		for i, link := range object.Links {
			node.Links[i] = Link{
				Hash: link.Hash.B58String(),
				Name: link.Name,
				Size: link.Size,
			}
		}

		return node, nil
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
		cmds.StringArg("key", true, false, "Key of the object to retrieve (in base58-encoded multihash format)"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key := req.Arguments()[0]

		object, err := objectGet(n, key)
		if err != nil {
			return nil, err
		}

		ns, err := object.Stat()
		if err != nil {
			return nil, err
		}

		return ns, nil
	},
	Type: dag.NodeStat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ns := res.Output().(*dag.NodeStat)

			var buf bytes.Buffer
			w := func(s string, n int) {
				buf.Write([]byte(fmt.Sprintf("%s: %d\n", s, n)))
			}
			w("NumLinks", ns.NumLinks)
			w("BlockSize", ns.BlockSize)
			w("LinksSize", ns.LinksSize)
			w("DataSize", ns.DataSize)
			w("CumulativeSize", ns.CumulativeSize)

			return &buf, nil
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

Data should be in the format specified by <encoding>.
<encoding> may be one of the following:
	* "protobuf"
	* "json"
`,
	},

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a DAG object"),
		cmds.StringArg("encoding", true, false, "Encoding type of <data>, either \"protobuf\" or \"json\""),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		input, err := req.Files().NextFile()
		if err != nil && err != io.EOF {
			return nil, err
		}

		encoding := req.Arguments()[0]

		output, err := objectPut(n, input, encoding)
		if err != nil {
			errType := cmds.ErrNormal
			if err == ErrUnknownObjectEnc {
				errType = cmds.ErrClient
			}
			return nil, cmds.Error{err.Error(), errType}
		}

		return output, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			object := res.Output().(*Object)
			return strings.NewReader("added " + object.Hash), nil
		},
	},
	Type: Object{},
}

// objectData takes a key string and writes out the raw bytes of that node (if there is one)
func objectData(n *core.IpfsNode, key string) (io.Reader, error) {
	dagnode, err := n.Resolver.ResolvePath(key)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectData: found dagnode %q (# of bytes: %d - # links: %d)", key, len(dagnode.Data), len(dagnode.Links))

	return bytes.NewReader(dagnode.Data), nil
}

// objectLinks takes a key string and lists the links it points to
func objectLinks(n *core.IpfsNode, key string) (*Object, error) {
	dagnode, err := n.Resolver.ResolvePath(key)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectLinks: found dagnode %q (# of bytes: %d - # links: %d)", key, len(dagnode.Data), len(dagnode.Links))

	return getOutput(dagnode)
}

// objectGet takes a key string from args and a format option and serializes the dagnode to that format
func objectGet(n *core.IpfsNode, key string) (*dag.Node, error) {
	dagnode, err := n.Resolver.ResolvePath(key)
	if err != nil {
		return nil, err
	}

	log.Debugf("objectGet: found dagnode %q (# of bytes: %d - # links: %d)", key, len(dagnode.Data), len(dagnode.Links))

	return dagnode, nil
}

// objectPut takes a format option, serializes bytes from stdin and updates the dag with that data
func objectPut(n *core.IpfsNode, input io.Reader, encoding string) (*Object, error) {
	var (
		dagnode *dag.Node
		data    []byte
		err     error
	)

	data, err = ioutil.ReadAll(io.LimitReader(input, inputLimit+10))
	if err != nil {
		return nil, err
	}

	if len(data) >= inputLimit {
		return nil, ErrObjectTooLarge
	}

	switch getObjectEnc(encoding) {
	case objectEncodingJSON:
		node := new(Node)
		err = json.Unmarshal(data, node)
		if err != nil {
			return nil, err
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

	err = addNode(n, dagnode)
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
	dagnode.Data = node.Data
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
