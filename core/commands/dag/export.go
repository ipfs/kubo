package dagcmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ipld "github.com/ipfs/go-ipld-format"
	mdag "github.com/ipfs/go-merkledag"

	cmds "github.com/ipfs/go-ipfs-cmds"
	gocar "github.com/ipld/go-car"
)

type dagExportNode struct {
	Err error
	Raw []byte
}

func dagExport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {

	c, err := cid.Decode(req.Arguments[0])
	if err != nil {
		return fmt.Errorf(
			"unable to parse root specification (currently only bare CIDs are supported): %s",
			err,
		)
	}

	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	// Code disabled until descent-issue in go-ipld-prime is fixed
	// https://github.com/ribasushi/gip-muddle-up
	//
	// sb := gipselectorbuilder.NewSelectorSpecBuilder(gipfree.NodeBuilder())
	// car := gocar.NewSelectiveCar(
	// 	req.Context,
	// 	<needs to be fixed to take format.NodeGetter as well>,
	// 	[]gocar.Dag{gocar.Dag{
	// 		Root: c,
	// 		Selector: sb.ExploreRecursive(
	// 			gipselector.RecursionLimitNone(),
	// 			sb.ExploreAll(sb.ExploreRecursiveEdge()),
	// 		).Node(),
	// 	}},
	// )
	// ...
	// if err := car.Write(pipeW); err != nil {}

	var buf bytes.Buffer
	iter, err := gocar.WriteCarIter(
		req.Context,
		mdag.NewSession(
			req.Context,
			api.Dag(),
		),
		[]cid.Cid{c},
		&buf,
	)
	if err != nil {
		return err
	}

	var cont bool
	for err, cont = iter(); cont; err, cont = iter() {
		den := dagExportNode{
			Err: err,
			Raw: buf.Bytes(),
		}
		if err2 := res.Emit(&den); err != nil {
			return err2
		}
		if err != nil {
			break
		}
		buf.Truncate(0)
	}

	// minimal user friendliness
	if err != nil &&
		err == ipld.ErrNotFound {
		explicitOffline, _ := req.Options["offline"].(bool)
		if explicitOffline {
			err = fmt.Errorf("%s (currently offline, perhaps retry without the offline flag)", err)
		} else {
			node, envErr := cmdenv.GetNode(env)
			if envErr == nil && !node.IsOnline {
				err = fmt.Errorf("%s (currently offline, perhaps retry after attaching to the network)", err)
			}
		}
	}

	return err
}

func finishCLIExport(res cmds.Response, re cmds.ResponseEmitter) error {
	pipeR, pipeW := io.Pipe()
	errCh := make(chan error, 2)
	go func() {
		defer func() {
			pipeW.Close()
			close(errCh)
		}()
		for v, err := res.Next(); v != nil; v, err = res.Next() {
			if err != nil {
				errCh <- re.CloseWithError(err)
				return
			}
			m := v.(map[string]interface{})
			if m["Err"] != nil {
				errCh <- m["Err"].(error)
				return
			}
			raw := m["Raw"].(string)
			dec, err := base64.StdEncoding.DecodeString(raw)
			if err != nil {
				errCh <- err
				return
			}
			pipeW.Write(dec)
		}
	}()
	re.Emit(pipeR)
	return <-errCh
}
