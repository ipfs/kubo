package tests

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	ipldcbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/stretchr/testify/require"
)

func newIPLDPath(t *testing.T, cid cid.Cid) path.ImmutablePath {
	p, err := path.NewPath(fmt.Sprintf("/%s/%s", path.IPLDNamespace, cid.String()))
	require.NoError(t, err)
	im, err := path.NewImmutablePath(p)
	require.NoError(t, err)
	return im
}

func (tp *TestSuite) TestPath(t *testing.T) {
	t.Run("TestMutablePath", tp.TestMutablePath)
	t.Run("TestPathRemainder", tp.TestPathRemainder)
	t.Run("TestEmptyPathRemainder", tp.TestEmptyPathRemainder)
	t.Run("TestInvalidPathRemainder", tp.TestInvalidPathRemainder)
	t.Run("TestPathRoot", tp.TestPathRoot)
	t.Run("TestPathJoin", tp.TestPathJoin)
}

func (tp *TestSuite) TestMutablePath(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	blk, err := api.Block().Put(ctx, strings.NewReader(`foo`))
	require.NoError(t, err)
	require.False(t, blk.Path().Mutable())
	require.NotNil(t, api.Key())

	keys, err := api.Key().List(ctx)
	require.NoError(t, err)
	require.True(t, keys[0].Path().Mutable())
}

func (tp *TestSuite) TestPathRemainder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)
	require.NotNil(t, api.Dag())

	nd, err := ipldcbor.FromJSON(strings.NewReader(`{"foo": {"bar": "baz"}}`), math.MaxUint64, -1)
	require.NoError(t, err)

	err = api.Dag().Add(ctx, nd)
	require.NoError(t, err)

	p, err := path.Join(path.FromCid(nd.Cid()), "foo", "bar")
	require.NoError(t, err)

	_, remainder, err := api.ResolvePath(ctx, p)
	require.NoError(t, err)
	require.Equal(t, "/foo/bar", path.SegmentsToString(remainder...))
}

func (tp *TestSuite) TestEmptyPathRemainder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)
	require.NotNil(t, api.Dag())

	nd, err := ipldcbor.FromJSON(strings.NewReader(`{"foo": {"bar": "baz"}}`), math.MaxUint64, -1)
	require.NoError(t, err)

	err = api.Dag().Add(ctx, nd)
	require.NoError(t, err)

	_, remainder, err := api.ResolvePath(ctx, path.FromCid(nd.Cid()))
	require.NoError(t, err)
	require.Empty(t, remainder)
}

func (tp *TestSuite) TestInvalidPathRemainder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)
	require.NotNil(t, api.Dag())

	nd, err := ipldcbor.FromJSON(strings.NewReader(`{"foo": {"bar": "baz"}}`), math.MaxUint64, -1)
	require.NoError(t, err)

	err = api.Dag().Add(ctx, nd)
	require.NoError(t, err)

	p, err := path.Join(newIPLDPath(t, nd.Cid()), "/bar/baz")
	require.NoError(t, err)

	_, _, err = api.ResolvePath(ctx, p)
	require.NotNil(t, err)
	require.ErrorContains(t, err, `no link named "bar"`)
}

func (tp *TestSuite) TestPathRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)
	require.NotNil(t, api.Block())

	blk, err := api.Block().Put(ctx, strings.NewReader(`foo`), options.Block.Format("raw"))
	require.NoError(t, err)
	require.NotNil(t, api.Dag())

	nd, err := ipldcbor.FromJSON(strings.NewReader(`{"foo": {"/": "`+blk.Path().RootCid().String()+`"}}`), math.MaxUint64, -1)
	require.NoError(t, err)

	err = api.Dag().Add(ctx, nd)
	require.NoError(t, err)

	p, err := path.Join(newIPLDPath(t, nd.Cid()), "/foo")
	require.NoError(t, err)

	rp, _, err := api.ResolvePath(ctx, p)
	require.NoError(t, err)
	require.Equal(t, rp.RootCid().String(), blk.Path().RootCid().String())
}

func (tp *TestSuite) TestPathJoin(t *testing.T) {
	p1, err := path.NewPath("/ipfs/QmYNmQKp6SuaVrpgWRsPTgCQCnpxUYGq76YEKBXuj2N4H6/bar/baz")
	require.NoError(t, err)

	p2, err := path.Join(p1, "foo")
	require.NoError(t, err)

	require.Equal(t, "/ipfs/QmYNmQKp6SuaVrpgWRsPTgCQCnpxUYGq76YEKBXuj2N4H6/bar/baz/foo", p2.String())
}
