package dagcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"

	node "gx/ipfs/QmU7bFWQ793qmvNy7outdCaMfSDNk8uqhx4VNrxYj5fj5g/go-ipld-node"
	cid "gx/ipfs/QmXfiyr2RWEXpVDdaYnD2HNiBk6UBddsvEP4RPfXb6nGqY/go-cid"
	ipldcbor "gx/ipfs/QmYRzW9YDHVNCDbfFzbS7TEXAG1swE1yjq1basZ5WnJYH4/go-ipld-cbor"
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
		cmds.StringOption("format", "f", "Format that the object will be.").Default("cbor"),
		cmds.StringOption("input-enc", "Format that the object will be.").Default("json"),
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
		_ = format
		switch ienc {
		case "json":
			var obj map[string]interface{}
			err := json.NewDecoder(fi).Decode(&obj)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			nd, err := convertJsonToType(obj, format)
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
			/*
				case "btc":
					data, err := ioutil.ReadAll(fi)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}

					blk, err := ipldbtc.DecodeBlock(data)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}

					c, err := n.DAG.Add(blk)
					if err != nil {
						res.SetError(err, cmds.ErrNormal)
						return
					}

					res.SetOutput(&OutputObject{Cid: c})
					return
			*/
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

func convertJsonToType(obj map[string]interface{}, format string) (node.Node, error) {
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
