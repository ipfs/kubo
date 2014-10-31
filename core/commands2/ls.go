package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
)

type Link struct {
	Name, Hash string
	Size       uint64
}

var ls = &cmds.Command{
	Help: "TODO",
	Run: func(res cmds.Response, req cmds.Request) {
		node := req.Context().Node
		output := make(map[string][]Link, len(req.Arguments()))

		for _, path := range req.Arguments() {
			dagnode, err := node.Resolver.ResolvePath(path)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			output[path] = make([]Link, len(dagnode.Links))
			for i, link := range dagnode.Links {
				output[path][i] = Link{
					Name: link.Name,
					Hash: link.Hash.B58String(),
					Size: link.Size,
				}
			}
		}

		res.SetValue(output)
	},
	Format: func(res cmds.Response) (string, error) {
		s := ""
		output := res.Value().(*map[string][]Link)

		for path, links := range *output {
			if len(*output) > 1 {
				s += fmt.Sprintf("%s:\n", path)
			}

			for _, link := range links {
				s += fmt.Sprintf("-> %s %s (%v bytes)\n", link.Name, link.Hash, link.Size)
			}

			if len(*output) > 1 {
				s += "\n"
			}
		}

		return s, nil
	},
	Type: &map[string][]Link{},
}
