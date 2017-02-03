package commands

import (
	"context"
	"fmt"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/filestore"
	u "gx/ipfs/QmZuY8aV7zbNXVy6DyN9SmnuH3o9nG852F4aTiSBpts8d1/go-ipfs-util"
)

var FileStoreCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filestore objects.",
	},
	Subcommands: map[string]*cmds.Command{
		"ls":     lsFileStore,
		"verify": verifyFileStore,
	},
}

var lsFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List objects in filestore.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := getFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		next, err := filestore.ListAll(fs)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		out := listResAsChan(next, req.Context())
		res.SetOutput(out)
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)
		errors := false
		for r0 := range outChan {
			r := r0.(*filestore.ListRes)
			if r.ErrorMsg != "" {
				fmt.Fprintf(res.Stderr(), "%s\n", r.ErrorMsg)
			} else {
				fmt.Fprintf(res.Stdout(), "%s\n", r.FormatLong())
			}
		}
		if errors {
			res.SetError(fmt.Errorf("errors while displaying some entries"), cmds.ErrNormal)
		}
	},
	Type: filestore.ListRes{},
}

var verifyFileStore = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Verify objects in filestore.",
	},
	Run: func(req cmds.Request, res cmds.Response) {
		_, fs, err := getFilestore(req)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		next, err := filestore.VerifyAll(fs)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		out := listResAsChan(next, req.Context())
		res.SetOutput(out)
	},
	PostRun: func(req cmds.Request, res cmds.Response) {
		if res.Error() != nil {
			return
		}
		outChan, ok := res.Output().(<-chan interface{})
		if !ok {
			res.SetError(u.ErrCast(), cmds.ErrNormal)
			return
		}
		res.SetOutput(nil)
		for r0 := range outChan {
			r := r0.(*filestore.ListRes)
			fmt.Fprintf(res.Stdout(), "%s %s\n", r.Status.Format(), r.FormatLong())
		}
	},
	Type: filestore.ListRes{},
}

func getFilestore(req cmds.Request) (*core.IpfsNode, *filestore.Filestore, error) {
	n, err := req.InvocContext().GetNode()
	if err != nil {
		return nil, nil, err
	}
	fs := n.Filestore
	if fs == nil {
		return n, nil, fmt.Errorf("filestore not enabled")
	}
	return n, fs, err
}

func listResAsChan(next func() *filestore.ListRes, ctx context.Context) <-chan interface{} {
	out := make(chan interface{}, 128)
	go func() {
		defer close(out)
		for {
			r := next()
			if r == nil {
				return
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
