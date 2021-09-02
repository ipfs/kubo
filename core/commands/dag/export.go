package dagcmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cheggaaa/pb"
	cid "github.com/ipfs/go-cid"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	ipld "github.com/ipfs/go-ipld-format"
	mdag "github.com/ipfs/go-merkledag"

	cmds "github.com/ipfs/go-ipfs-cmds"
	gocar "github.com/ipld/go-car"
)

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

	pipeR, pipeW := io.Pipe()

	errCh := make(chan error, 2) // we only report the 1st error
	go func() {
		defer func() {
			if err := pipeW.Close(); err != nil {
				errCh <- fmt.Errorf("stream flush failed: %s", err)
			}
			close(errCh)
		}()

		if err := gocar.WriteCar(
			req.Context,
			mdag.NewSession(
				req.Context,
				api.Dag(),
			),
			[]cid.Cid{c},
			pipeW,
		); err != nil {
			errCh <- err
		}
	}()

	if err := res.Emit(pipeR); err != nil {
		pipeR.Close() // ignore the error if any
		return err
	}

	err = <-errCh

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

	var showProgress bool
	val, specified := res.Request().Options[progressOptionName]
	if !specified {
		// default based on TTY availability
		errStat, _ := os.Stderr.Stat()
		if 0 != (errStat.Mode() & os.ModeCharDevice) {
			showProgress = true
		}
	} else if val.(bool) {
		showProgress = true
	}

	// simple passthrough, no progress
	if !showProgress {
		return cmds.Copy(re, res)
	}

	bar := pb.New64(0).SetUnits(pb.U_BYTES)
	bar.Output = os.Stderr
	bar.ShowSpeed = true
	bar.ShowElapsedTime = true
	bar.RefreshRate = 500 * time.Millisecond
	bar.Start()

	var processedOneResponse bool
	for {
		v, err := res.Next()
		if err == io.EOF {

			// We only write the final bar update on success
			// On error it looks too weird
			bar.Finish()

			return re.Close()
		} else if err != nil {
			return re.CloseWithError(err)
		} else if processedOneResponse {
			return re.CloseWithError(errors.New("unexpected multipart response during emit, please file a bugreport"))
		}

		r, ok := v.(io.Reader)
		if !ok {
			// some sort of encoded response, this should not be happening
			return errors.New("unexpected non-stream passed to PostRun: please file a bugreport")
		}

		processedOneResponse = true

		if err := re.Emit(bar.NewProxyReader(r)); err != nil {
			return err
		}
	}
}
