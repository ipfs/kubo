package commands

import (
	"fmt"
	"strings"

	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"
	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []string
}

// KeyListTextMarshaler outputs a KeyList as plaintext, one key per line
func KeyListTextMarshaler(res cmds.Response) ([]byte, error) {
	output := res.Output().(*KeyList)
	s := strings.Join(output.Keys, "\n")
	return []byte(s), nil
}

var refsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Lists link hashes from an object",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and displays the link
hashes it contains, with the following format:

  <link base58 hash>

Note: list all refs recursively with -r.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to the object(s) to list refs from"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("unique", "u", "Omit duplicate refs from output"),
		cmds.BoolOption("recursive", "r", "Recursively list links of child nodes"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		unique, found, err := req.Option("unique").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			unique = false
		}

		recursive, found, err := req.Option("recursive").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			recursive = false
		}

		return getRefs(n, req.Arguments(), unique, recursive)
	},
	Type: &KeyList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: KeyListTextMarshaler,
	},
}

func getRefs(n *core.IpfsNode, paths []string, unique, recursive bool) (*KeyList, error) {
	var refsSeen map[u.Key]bool
	if unique {
		refsSeen = make(map[u.Key]bool)
	}

	refs := make([]string, 0)

	for _, path := range paths {
		object, err := n.Resolver.ResolvePath(path)
		if err != nil {
			return nil, err
		}

		refs, err = addRefs(n, object, refs, refsSeen, recursive)
		if err != nil {
			return nil, err
		}
	}

	return &KeyList{refs}, nil
}

func addRefs(n *core.IpfsNode, object *dag.Node, refs []string, refsSeen map[u.Key]bool, recursive bool) ([]string, error) {
	for _, link := range object.Links {
		var found bool
		found, refs = addRef(link.Hash, refs, refsSeen)

		if recursive && !found {
			child, err := n.DAG.Get(u.Key(link.Hash))
			if err != nil {
				return nil, fmt.Errorf("cannot retrieve %s (%s)", link.Hash.B58String(), err)
			}

			refs, err = addRefs(n, child, refs, refsSeen, recursive)
			if err != nil {
				return nil, err
			}
		}
	}

	return refs, nil
}

func addRef(h mh.Multihash, refs []string, refsSeen map[u.Key]bool) (bool, []string) {
	if refsSeen != nil {
		_, found := refsSeen[u.Key(h)]
		if found {
			return true, refs
		}
		refsSeen[u.Key(h)] = true
	}

	refs = append(refs, h.B58String())
	return false, refs
}
