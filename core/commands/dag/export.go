package dagcmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cheggaaa/pb/v3"
	blockstore "github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/boxo/dag/walker"
	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/kubo/core/commands/cmdenv"
	"github.com/ipfs/kubo/core/commands/cmdutils"
	iface "github.com/ipfs/kubo/core/coreiface"
	gocar "github.com/ipld/go-car/v2"
	carstorage "github.com/ipld/go-car/v2/storage"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	selectorparse "github.com/ipld/go-ipld-prime/traversal/selector/parse"
)

// pb/v3 template for `ipfs dag export`: byte counter, speed, and
// elapsed time. No bar/percent/ETA because the total size of the
// CAR stream is not known up front. The explicit "%s/s" speed
// format overrides pb's default "p/s" suffix so the rate renders
// as "MiB/s".
const progressBarTemplate = `{{counters . }} {{speed . "%s/s" "?/s"}} {{etime . }}`

func dagExport(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
	// Accept CID or a content path
	p, err := cmdutils.PathOrCidPath(req.Arguments[0])
	if err != nil {
		return err
	}

	localOnly, _ := req.Options[localOnlyOptionName].(bool)
	if localOnly {
		offlineVal, offlineSet := req.Options["offline"].(bool)
		if offlineSet && !offlineVal {
			return fmt.Errorf("--%s implies --offline and cannot be combined with --offline=false; please drop one of them", localOnlyOptionName)
		}
		// --local-only implies --offline: a partial CAR is local-only by
		// definition, so missing blocks must not be fetched over the network.
		req.Options["offline"] = true
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

	var bs blockstore.Blockstore
	if localOnly {
		node, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}
		bs = node.Blockstore
	}

	pipeR, pipeW := io.Pipe()

	errCh := make(chan error, 2) // we only report the 1st error
	go func() {
		defer func() {
			if err := pipeW.Close(); err != nil {
				errCh <- fmt.Errorf("stream flush failed: %s", err)
			}
			close(errCh)
		}()

		if localOnly {
			if err := exportPartialCAR(req.Context, bs, c, pipeW); err != nil {
				errCh <- err
			}
			return
		}

		lsys := cidlink.DefaultLinkSystem()
		lsys.SetReadStorage(&dagStore{dag: api.Dag(), ctx: req.Context})

		// Uncomment the following to support CARv2 output.
		/*
			car, err := gocar.NewSelectiveWriter(req.Context, &lsys, c, selectorparse.CommonSelector_ExploreAllRecursively, gocar.AllowDuplicatePuts(false))
			if err != nil {
				errCh <- err
				return
			}
			if _, err = car.WriteTo(pipeW); err != nil {
				errCh <- err
				return
			}
		*/
		_, err := gocar.TraverseV1(req.Context, &lsys, c, selectorparse.CommonSelector_ExploreAllRecursively, pipeW, gocar.AllowDuplicatePuts(false))
		if err != nil {
			errCh <- err
			return
		}

	}()

	res.SetEncodingType(cmds.OctetStream)
	res.SetContentType("application/vnd.ipld.car")
	if err := res.Emit(pipeR); err != nil {
		pipeR.Close() // ignore the error if any
		return err
	}

	err = <-errCh

	// minimal user friendliness
	if errors.Is(err, ipld.ErrNotFound{}) {
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

// exportPartialCAR is the best-effort engine behind `dag export --local-only`.
// It walks the DAG rooted at root using only the local blockstore and writes
// the visited blocks to w as a CARv1 stream. Any block that is missing or
// unreadable locally (and its entire subtree) is treated as "not available
// locally" and skipped. The resulting CAR is therefore partial by design.
//
// Errors writing the CAR itself (i.e. emit failures) are surfaced — those
// are output problems, not local-availability problems.
//
// This mirrors the MFS+unique provider in core/node/provider.go.
func exportPartialCAR(ctx context.Context, bs blockstore.Blockstore, root cid.Cid, w io.Writer) error {
	writable, err := carstorage.NewWritable(w, []cid.Cid{root}, gocar.WriteAsCarV1(true))
	if err != nil {
		return err
	}

	// Capture the first emit (write-side) error so the walk stops cleanly.
	var emitErr error
	emit := func(k cid.Cid) bool {
		blk, err := bs.Get(ctx, k)
		if err != nil {
			// Any read error after locality passed (e.g. GC race,
			// corruption) is treated as "not available locally" — skip
			// the block and keep streaming the rest of the partial CAR.
			return true
		}
		if err := writable.Put(ctx, k.KeyString(), blk.RawData()); err != nil {
			emitErr = err
			return false
		}
		return true
	}

	// walker.WithLocality + walker.LinksFetcherFromBlockstore also skip-and-log
	// on locality/fetch errors, which matches the best-effort semantics here.
	if err := walker.WalkDAG(ctx, root,
		walker.LinksFetcherFromBlockstore(bs),
		emit,
		walker.WithLocality(func(ctx context.Context, k cid.Cid) (bool, error) { return bs.Has(ctx, k) }),
	); err != nil {
		return err
	}
	return emitErr
}

func finishCLIExport(res cmds.Response, re cmds.ResponseEmitter) error {
	if !cmdenv.ShouldShowProgress(res.Request(), progressOptionName) {
		return cmds.Copy(re, res)
	}

	bar := pb.New64(0).Set(pb.Bytes, true).SetWriter(os.Stderr).SetRefreshRate(500 * time.Millisecond)
	bar.SetTemplateString(progressBarTemplate)
	bar.Start()

	var processedOneResponse bool
	for {
		v, err := res.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				// We only write the final bar update on success
				// On error it looks too weird
				bar.Finish()
				return re.Close()
			}
			return re.CloseWithError(err)
		}

		if processedOneResponse {
			return re.CloseWithError(errors.New("unexpected multipart response during emit, please file a bugreport"))
		}

		r, ok := v.(io.Reader)
		if !ok {
			// some sort of encoded response, this should not be happening
			return errors.New("unexpected non-stream passed to PostRun: please file a bugreport")
		}

		processedOneResponse = true

		if err = re.Emit(bar.NewProxyReader(r)); err != nil {
			return err
		}
	}
}

type dagStore struct {
	dag iface.APIDagService
	ctx context.Context
}

func (ds *dagStore) Get(ctx context.Context, key string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	c, err := cidFromBinString(key)
	if err != nil {
		return nil, err
	}

	block, err := ds.dag.Get(ds.ctx, c)
	if err != nil {
		return nil, err
	}

	return block.RawData(), nil
}

func (ds *dagStore) Has(ctx context.Context, key string) (bool, error) {
	_, err := ds.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ipld.ErrNotFound{}) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func cidFromBinString(key string) (cid.Cid, error) {
	l, k, err := cid.CidFromBytes([]byte(key))
	if err != nil {
		return cid.Undef, fmt.Errorf("dagStore: key was not a cid: %w", err)
	}
	if l != len(key) {
		return cid.Undef, fmt.Errorf("dagStore: key was not a cid: had %d bytes leftover", len(key)-l)
	}
	return k, nil
}
