package commands

import (
	"bytes"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"

	context "context"
	cid "gx/ipfs/QmV5gPoRsjN1Gid3LMdNZTyfCtP2DsvqEbMAmz82RmmiGk/go-cid"
	u "gx/ipfs/QmZuY8aV7zbNXVy6DyN9SmnuH3o9nG852F4aTiSBpts8d1/go-ipfs-util"
)

var PinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pin (and unpin) objects to local storage.",
	},

	Subcommands: map[string]*cmds.Command{
		"add": addPinCmd,
		"rm":  rmPinCmd,
		"ls":  listPinCmd,
	},
}

type PinOutput struct {
	Pins []string
}

type AddPinOutput struct {
	Pins     []string
	Progress int `json:",omitempty"`
}

var addPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Pin objects to local storage.",
		ShortDescription: "Stores an IPFS object(s) from a given path locally to disk.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively pin the object linked to by the specified object(s).").Default(true),
		cmds.BoolOption("progress", "Show progress"),
	},
	Type: AddPinOutput{},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		defer n.Blockstore.PinLock().Unlock()

		// set recursive flag
		recursive, _, err := req.Option("recursive").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		showProgress, _, _ := req.Option("progress").Bool()

		if !showProgress {
			added, err := corerepo.Pin(n, req.Context(), req.Arguments(), recursive)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			res.SetOutput(&AddPinOutput{Pins: cidsToStrings(added)})
			return
		}

		v := new(dag.ProgressTracker)
		ctx := v.DeriveContext(req.Context())

		ch := make(chan []*cid.Cid)
		go func() {
			defer close(ch)
			added, err := corerepo.Pin(n, ctx, req.Arguments(), recursive)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
			ch <- added
		}()
		out := make(chan interface{})
		res.SetOutput((<-chan interface{})(out))
		go func() {
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			defer close(out)
			for {
				select {
				case val, ok := <-ch:
					if !ok {
						// error already set just return
						return
					}
					if pv := v.Value(); pv != 0 {
						out <- &AddPinOutput{Progress: v.Value()}
					}
					out <- &AddPinOutput{Pins: cidsToStrings(val)}
					return
				case <-ticker.C:
					out <- &AddPinOutput{Progress: v.Value()}
				case <-ctx.Done():
					res.SetError(ctx.Err(), cmds.ErrNormal)
					return
				}
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			var added []string

			switch out := res.Output().(type) {
			case *AddPinOutput:
				added = out.Pins
			case <-chan interface{}:
				progressLine := false
				for r0 := range out {
					r := r0.(*AddPinOutput)
					if r.Pins != nil {
						added = r.Pins
					} else {
						if progressLine {
							fmt.Fprintf(res.Stderr(), "\r")
						}
						fmt.Fprintf(res.Stderr(), "Fetched/Processed %d nodes", r.Progress)
						progressLine = true
					}
				}
				if progressLine {
					fmt.Fprintf(res.Stderr(), "\n")
				}
				if res.Error() != nil {
					return nil, res.Error()
				}
			default:
				return nil, u.ErrCast()
			}
			var pintype string
			rec, found, _ := res.Request().Option("recursive").Bool()
			if rec || !found {
				pintype = "recursively"
			} else {
				pintype = "directly"
			}

			buf := new(bytes.Buffer)
			for _, k := range added {
				fmt.Fprintf(buf, "pinned %s %s\n", k, pintype)
			}
			return buf, nil
		},
	},
}

var rmPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Remove pinned objects from local storage.",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed. (By default, recursively. Use -r=false for direct pins.)
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively unpin the object linked to by the specified object(s).").Default(true),
	},
	Type: PinOutput{},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// set recursive flag
		recursive, _, err := req.Option("recursive").Bool()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		removed, err := corerepo.Unpin(n, req.Context(), req.Arguments(), recursive)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&PinOutput{cidsToStrings(removed)})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			added, ok := res.Output().(*PinOutput)
			if !ok {
				return nil, u.ErrCast()
			}

			buf := new(bytes.Buffer)
			for _, k := range added.Pins {
				fmt.Fprintf(buf, "unpinned %s\n", k)
			}
			return buf, nil
		},
	},
}

var listPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects pinned to local storage.",
		ShortDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.
`,
		LongDescription: `
Returns a list of objects that are pinned locally.
By default, all pinned objects are returned, but the '--type' flag or
arguments can restrict that to a specific pin type or to some specific objects
respectively.

Use --type=<type> to specify the type of pinned keys to list.
Valid values are:
    * "direct": pin that specific object.
    * "recursive": pin that specific object, and indirectly pin all its
    	descendants
    * "indirect": pinned indirectly by an ancestor (like a refcount)
    * "all"

With arguments, the command fails if any of the arguments is not a pinned
object. And if --type=<type> is additionally used, the command will also fail
if any of the arguments is not of the specified type.

Example:
	$ echo "hello" | ipfs add -q
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin ls
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN recursive
	# now remove the pin, and repin it directly
	$ ipfs pin rm QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	unpinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	$ ipfs pin add -r=false QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	pinned QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN directly
	$ ipfs pin ls --type=direct
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
	$ ipfs pin ls QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
	QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN direct
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", false, true, "Path to object(s) to be listed."),
	},
	Options: []cmds.Option{
		cmds.StringOption("type", "t", "The type of pinned keys to list. Can be \"direct\", \"indirect\", \"recursive\", or \"all\".").Default("all"),
		cmds.BoolOption("quiet", "q", "Write just hashes of objects.").Default(false),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		typeStr, _, err := req.Option("type").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		switch typeStr {
		case "all", "direct", "indirect", "recursive":
		default:
			err = fmt.Errorf("Invalid type '%s', must be one of {direct, indirect, recursive, all}", typeStr)
			res.SetError(err, cmds.ErrClient)
			return
		}

		var keys map[string]RefKeyObject

		if len(req.Arguments()) > 0 {
			keys, err = pinLsKeys(req.Arguments(), typeStr, req.Context(), n)
		} else {
			keys, err = pinLsAll(typeStr, req.Context(), n)
		}

		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		} else {
			res.SetOutput(&RefKeyList{Keys: keys})
		}
	},
	Type: RefKeyList{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			quiet, _, err := res.Request().Option("quiet").Bool()
			if err != nil {
				return nil, err
			}

			keys, ok := res.Output().(*RefKeyList)
			if !ok {
				return nil, u.ErrCast()
			}
			out := new(bytes.Buffer)
			for k, v := range keys.Keys {
				if quiet {
					fmt.Fprintf(out, "%s\n", k)
				} else {
					fmt.Fprintf(out, "%s %s\n", k, v.Type)
				}
			}
			return out, nil
		},
	},
}

type RefKeyObject struct {
	Type string
}

type RefKeyList struct {
	Keys map[string]RefKeyObject
}

func pinLsKeys(args []string, typeStr string, ctx context.Context, n *core.IpfsNode) (map[string]RefKeyObject, error) {

	mode, ok := pin.StringToPinMode(typeStr)
	if !ok {
		return nil, fmt.Errorf("invalid pin mode '%s'", typeStr)
	}

	keys := make(map[string]RefKeyObject)

	for _, p := range args {
		pth, err := path.ParsePath(p)
		if err != nil {
			return nil, err
		}

		c, err := core.ResolveToCid(ctx, n, pth)
		if err != nil {
			return nil, err
		}

		pinType, pinned, err := n.Pinning.IsPinnedWithType(c, mode)
		if err != nil {
			return nil, err
		}

		if !pinned {
			return nil, fmt.Errorf("path '%s' is not pinned", p)
		}

		switch pinType {
		case "direct", "indirect", "recursive", "internal":
		default:
			pinType = "indirect through " + pinType
		}
		keys[c.String()] = RefKeyObject{
			Type: pinType,
		}
	}

	return keys, nil
}

func pinLsAll(typeStr string, ctx context.Context, n *core.IpfsNode) (map[string]RefKeyObject, error) {

	keys := make(map[string]RefKeyObject)

	AddToResultKeys := func(keyList []*cid.Cid, typeStr string) {
		for _, c := range keyList {
			keys[c.String()] = RefKeyObject{
				Type: typeStr,
			}
		}
	}

	if typeStr == "direct" || typeStr == "all" {
		AddToResultKeys(n.Pinning.DirectKeys(), "direct")
	}
	if typeStr == "indirect" || typeStr == "all" {
		set := cid.NewSet()
		for _, k := range n.Pinning.RecursiveKeys() {
			err := dag.EnumerateChildren(n.Context(), n.DAG.GetLinks, k, set.Visit)
			if err != nil {
				return nil, err
			}
		}
		AddToResultKeys(set.Keys(), "indirect")
	}
	if typeStr == "recursive" || typeStr == "all" {
		AddToResultKeys(n.Pinning.RecursiveKeys(), "recursive")
	}

	return keys, nil
}

func cidsToStrings(cs []*cid.Cid) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.String())
	}
	return out
}
