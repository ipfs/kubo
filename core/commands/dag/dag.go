package dagcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"

	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ipldcbor "gx/ipfs/QmY7L2aEa1rHjkSSbXJB8oC7825JTpUUvDygmM2JPQeqhr/go-ipld-cbor"
)

var DagCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Interact with ipld dag objects.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"put": DagPutCmd,
		"get": DagGetCmd,
	},
}

type OutputObject struct {
	Cid *cid.Cid
}

var DagPutCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Add a dag node to ipfs.",
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("object data", true, false, "The object to put").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("format", "f", "Format that the object will be added as.").Default("cbor"),
		cmds.StringOption("input-enc", "Format that the input object will be.").Default("json"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		ienc, _, _ := req.Option("input-enc").String()
		format, _, _ := req.Option("format").String()

		switch ienc {
		case "json":
			nd, err := convertJsonToType(fi, format)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			c, err := n.DAG.Add(nd)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			res.SetOutput(&OutputObject{Cid: c})
			return
		default:
			res.SetError(fmt.Errorf("unrecognized input encoding: %s", ienc), cmds.ErrNormal)
			return
		}
	},
	Type: OutputObject{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			oobj, ok := res.Output().(*OutputObject)
			if !ok {
				return nil, fmt.Errorf("expected a different object in marshaler")
			}

			return strings.NewReader(oobj.Cid.String()), nil
		},
	},
}

var DagGetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get a dag node from ipfs.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("cid", true, false, "The cid of the object to get").EnableStdin(),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		c, err := cid.Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		obj, err := n.DAG.Get(req.Context(), c)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(obj)
	},
}

func convertJsonToType(r io.Reader, format string) (node.Node, error) {
	var obj map[string]interface{}
	err := json.NewDecoder(r).Decode(&obj)
	if err != nil {
		return nil, err
	}

	switch format {
	case "cbor", "dag-cbor":
		return convertJsonToCbor(obj)
	case "dag-pb", "protobuf":
		return nil, fmt.Errorf("protobuf handling in 'dag' command not yet implemented")
	default:
		return nil, fmt.Errorf("unknown target format: %s", format)
	}
}

func convertJsonToCbor(from map[string]interface{}) (*ipldcbor.Node, error) {
	out, err := convertMapSIToCbor(from)
	if err != nil {
		return nil, err
	}

	return ipldcbor.WrapMap(out)
}

func convertMapSIToCbor(from map[string]interface{}) (map[interface{}]interface{}, error) {
	to := make(map[interface{}]interface{})
	for k, v := range from {
		out, err := convertToCborIshObj(v)
		if err != nil {
			return nil, err
		}
		to[k] = out
	}

	return to, nil
}

func convertToCborIshObj(i interface{}) (interface{}, error) {
	switch v := i.(type) {
	case map[string]interface{}:
		if lnk, ok := v["/"]; ok && len(v) == 1 {
			// special case for links
			vstr, ok := lnk.(string)
			if !ok {
				return nil, fmt.Errorf("link should have been a string")
			}

			c, err := cid.Decode(vstr)
			if err != nil {
				return nil, err
			}

			return &ipldcbor.Link{Target: c}, nil
		}

		return convertMapSIToCbor(v)
	case []interface{}:
		var out []interface{}
		for _, o := range v {
			obj, err := convertToCborIshObj(o)
			if err != nil {
				return nil, err
			}

			out = append(out, obj)
		}

		return out, nil
	default:
		return v, nil
	}
}
