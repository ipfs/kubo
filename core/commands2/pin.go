package commands

import (
	"errors"
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/merkledag"
)

var pinCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"recursive", "r"}, cmds.Bool},
		cmds.Option{[]string{"depth", "d"}, cmds.Uint},
	},
	Arguments: []cmds.Argument{
		cmds.Argument{"object", cmds.ArgString, true, true},
	},
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		// set recursive flag
		opt, _ := req.Option("recursive")
		recursive, _ := opt.(bool) // false if cast fails.

		/*depth := 1 // default (non recursive)

		// if recursive, set depth flag
		if recursive {
			opt, found := req.Option("depth")
			if d, ok := opt.(int); found && ok {
				depth = d
			} else {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}
		}*/

		paths := make([]string, 0)
		for _, arg := range req.Arguments() {
			path, ok := arg.(string)
			if !ok {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}
			paths = append(paths, path)
		}

		_, err := pin(n, paths, recursive)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}

		// TODO: create some output to show what got pinned
	},
}

var unpinCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.Option{[]string{"recursive", "r"}, cmds.Bool},
	},
	Arguments: []cmds.Argument{
		cmds.Argument{"object", cmds.ArgString, true, true},
	},
	Run: func(res cmds.Response, req cmds.Request) {
		n := req.Context().Node

		// set recursive flag
		opt, _ := req.Option("recursive")
		recursive, _ := opt.(bool) // false if cast fails.

		for _, arg := range req.Arguments() {
			path, ok := arg.(string)
			if !ok {
				res.SetError(errors.New("cast error"), cmds.ErrNormal)
				return
			}

			dagnode, err := n.Resolver.ResolvePath(path)
			if err != nil {
				res.SetError(fmt.Errorf("pin error: %v", err), cmds.ErrNormal)
				return
			}

			k, _ := dagnode.Key()
			err = n.Pinning.Unpin(k, recursive)
			if err != nil {
				res.SetError(fmt.Errorf("pin: %v", err), cmds.ErrNormal)
				return
			}
		}

		err := n.Pinning.Flush()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}

		// TODO: create some output to show what got unpinned
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
