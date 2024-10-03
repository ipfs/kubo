package dagcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cheggaaa/pb"
	blocks "github.com/ipfs/go-block-format"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	iface "github.com/ipfs/kubo/core/coreiface"

	cmds "github.com/ipfs/go-ipfs-cmds"
	gocar "github.com/ipld/go-car"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
)

func dagExport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	// Accept CID or a content path
	p, err := cmdutils.PathOrCidPath(req.Arguments[0])
	if err != nil {
		return err
	}

	api, err := cmdenv.GetApi(env, req)
	if err != nil {
		return err
	}

	// Resolve path and confirm the root block is available, fail fast if not
	b, err := api.Block().Stat(req.Context, p)
	if err != nil {
		return err
	}
	c := b.Path().RootCid()

	pipeR, pipeW := io.Pipe()

	errCh := make(chan error, 2) // we only report the 1st error
	go func() {
		defer func() {
			if err := pipeW.Close(); err != nil {
				errCh <- fmt.Errorf("stream flush failed: %s", err)
			}
			close(errCh)
		}()

		store := dagStore{dag: api.Dag(), ctx: req.Context}
		dag := gocar.Dag{Root: c, Selector: selectorparse.CommonSelector_ExploreAllRecursively}
		// TraverseLinksOnlyOnce is safe for an exhaustive selector but won't be when we allow
		// arbitrary selectors here
		car := gocar.NewSelectiveCar(req.Context, store, []gocar.Dag{dag}, gocar.TraverseLinksOnlyOnce())
		if err := car.Write(pipeW); err != nil {
			errCh <- err
		}
	}()

	if err := res.Emit(pipeR); err != nil {
		pipeR.Close() // ignore the error if any
		return err
	}

	err = <-errCh

	// minimal user friendliness
	if ipld.IsNotFound(err) {
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
		if (errStat.Mode() & os.ModeCharDevice) != 0 {
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

// FIXME(@Jorropo): https://github.com/ipld/go-car/issues/315
type dagStore struct {
	dag iface.APIDagService
	ctx context.Context
}

func (ds dagStore) Get(_ context.Context, c cid.Cid) (blocks.Block, error) {
	return ds.dag.Get(ds.ctx, c)
}
