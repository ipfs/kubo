package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	cmds "github.com/jbenet/go-ipfs/commands"
	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

// ErrObjectTooLarge is returned when too much data was read from stdin. current limit 512k
var ErrObjectTooLarge = errors.New("input object was too large. limit is 512kbytes")

const inputLimit = 512 * 1024

type Node struct {
	Links []Link
	Data  []byte
}

var objectCmd = &cmds.Command{
	Description: "Interact with ipfs objects",
	Help:        `'ipfs object' is a plumbing command used to manipulate DAG objects directly.`,

	Subcommands: map[string]*cmds.Command{
		"data":  objectDataCmd,
		"links": objectLinksCmd,
		"get":   objectGetCmd,
		"put":   objectPutCmd,
	},
}

var objectDataCmd = &cmds.Command{
	Description: "Outputs the raw bytes in an IPFS object",
	Help: `ipfs data is a plumbing command for retreiving the raw bytes stored in a DAG node.
It outputs to stdout, and <key> is a base58 encoded multihash.

Note that the "--encoding" option does not affect the output, since the
output is the raw data of the object.
`,

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key, ok := req.Arguments()[0].(string)
		if !ok {
			return nil, u.ErrCast()
		}

		return objectData(n, key)
	},
}

var objectLinksCmd = &cmds.Command{
	Description: "Outputs the links pointed to by the specified object",
	Help: `'ipfs block get' is a plumbing command for retreiving raw IPFS blocks.
It outputs to stdout, and <key> is a base58 encoded multihash.`,

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve, in base58-encoded multihash format"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key, ok := req.Arguments()[0].(string)
		if !ok {
			return nil, u.ErrCast()
		}

		return objectLinks(n, key)
	},
	Type: &Object{},
}

var objectGetCmd = &cmds.Command{
	Description: "Get and serialize the DAG node named by <key>",
	Help: `'ipfs object get' is a plumbing command for retreiving DAG nodes.
It serializes the DAG node to the format specified by the "--encoding" flag.
It outputs to stdout, and <key> is a base58 encoded multihash.

This command outputs data in the following encodings:
  * "protobuf"
  * "json"
  * "xml"
(Specified by the "--encoding" or "-enc" flags)`,

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "Key of the object to retrieve\n(in base58-encoded multihash format)"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		key, ok := req.Arguments()[0].(string)
		if !ok {
			return nil, u.ErrCast()
		}

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
	Type: &Node{},
	Marshallers: map[cmds.EncodingType]cmds.Marshaller{
		cmds.EncodingType("protobuf"): func(res cmds.Response) ([]byte, error) {
			object := res.Output().(*dag.Node)
			return object.Marshal()
		},
	},
}

var objectPutCmd = &cmds.Command{
	Description: "Stores input as a DAG object, outputs its key",
	Help: `'ipfs object put' is a plumbing command for storing DAG nodes.
It reads from stdin, and the output is a base58 encoded multihash.

Data should be in the format specified by <encoding>.
<encoding> may be one of the following:
  * "protobuf"
  * "json"
`,

	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "Data to be stored as a DAG object\nMust be encoded as specified in <encoding>"),
		cmds.StringArg("encoding", true, false, "Encoding type of <data>, either \"protobuf\" or \"json\""),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		input, ok := req.Arguments()[0].(io.Reader)
		if !ok {
			return nil, u.ErrCast()
		}

		encoding, ok := req.Arguments()[1].(string)
		if !ok {
			return nil, u.ErrCast()
		}

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
	Type: &Object{},
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
		dagnode = new(dag.Node)
		err = json.Unmarshal(data, dagnode)

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
