package commands

import (
	"bytes"
	"fmt"
	"io"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

var PinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to local storage",
	},

	Subcommands: map[string]*cmds.Command{
		"add": addPinCmd,
		"rm":  rmPinCmd,
		"ls":  listPinCmd,
	},
}

type PinOutput struct {
	Pinned []u.Key
}

var addPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pins objects to local storage",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and stores it locally
on disk.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively pin the object linked to by the specified object(s)"),
	},
	Type: PinOutput{},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// set recursive flag
		recursive, found, err := req.Option("recursive").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			recursive = false
		}

		nodes, err := pin(n, req.Arguments(), recursive)
		if err != nil {
			return nil, err
		}

		var added []u.Key
		for _, node := range nodes {
			k, err := node.Key()
			if err != nil {
				return nil, err
			}
			added = append(added, k)
		}

		// TODO: create some output to show what got pinned
		return &PinOutput{added}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			added, ok := res.Output().(*PinOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			buf := new(bytes.Buffer)
			for _, k := range added.Pinned {
				fmt.Fprintf(buf, "Pinned %s\n", k)
			}
			return buf, nil
		},
	},
}

var rmPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Unpin an object from local storage",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively unpin the object linked to by the specified object(s)"),
	},
	Type: PinOutput{},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// set recursive flag
		recursive, found, err := req.Option("recursive").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			recursive = false // default
		}

		nodes, err := unpin(n, req.Arguments(), recursive)
		if err != nil {
			return nil, err
		}

		var removed []u.Key
		for _, node := range nodes {
			k, err := node.Key()
			if err != nil {
				return nil, err
			}
			removed = append(removed, k)
		}

		return &PinOutput{removed}, nil
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			added, ok := res.Output().(*PinOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			buf := new(bytes.Buffer)
			for _, k := range added.Pinned {
				fmt.Fprintf(buf, "Unpinned %s\n", k)
			}
			return buf, nil
		},
	},
}

var listPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects pinned to local storage",
		ShortDescription: `
Returns a list of hashes of objects being pinned. Objects that are indirectly
or recursively pinned are not included in the list.
`,
		LongDescription: `
Returns a list of hashes of objects being pinned. Objects that are indirectly
or recursively pinned are not included in the list.

Use --type=<type> to specify the type of pinned keys to list. Valid values are:
    * "direct"
    * "indirect"
    * "recursive"
    * "all"
(Defaults to "direct")
`,
	},

	Options: []cmds.Option{
		cmds.StringOption("type", "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\". Defaults to \"direct\""),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		typeStr, found, err := req.Option("type").String()
		if err != nil {
			return nil, err
		}
		if !found {
			typeStr = "direct"
		}

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			return nil, cmds.ClientError("Invalid type '" + typeStr + "', must be one of {direct, indirect, recursive, all}")
		}

		keys := make([]u.Key, 0)
		if typeStr == "direct" || typeStr == "all" {
			keys = append(keys, n.Pinning.DirectKeys()...)
		}
		if typeStr == "indirect" || typeStr == "all" {
			keys = append(keys, n.Pinning.IndirectKeys()...)
		}
		if typeStr == "recursive" || typeStr == "all" {
			keys = append(keys, n.Pinning.RecursiveKeys()...)
		}

		return &KeyList{Keys: keys}, nil
	},
	Type: KeyList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: KeyListTextMarshaler,
	},
}

func pin(n *core.IpfsNode, paths []string, recursive bool) ([]*merkledag.Node, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, path := range paths {
		dagnode, err := n.Resolver.ResolvePath(path)
		if err != nil {
			return nil, fmt.Errorf("pin error: %v", err)
		}
		dagnodes = append(dagnodes, dagnode)
	}

	for _, dagnode := range dagnodes {
		err := n.Pinning.Pin(dagnode, recursive)
		if err != nil {
			return nil, fmt.Errorf("pin: %v", err)
		}
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}

	return dagnodes, nil
}

func unpin(n *core.IpfsNode, paths []string, recursive bool) ([]*merkledag.Node, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, path := range paths {
		dagnode, err := n.Resolver.ResolvePath(path)
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, dagnode)
	}

	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()
		err := n.Pinning.Unpin(k, recursive)
		if err != nil {
			return nil, err
		}
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}
	return dagnodes, nil
}
