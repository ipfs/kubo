package tests

import (
	"context"
	"testing"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/path"
	ipld "github.com/ipfs/go-ipld-format"
	iface "github.com/ipfs/kubo/core/coreiface"
	opt "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/stretchr/testify/require"
)

func (tp *TestSuite) TestObject(t *testing.T) {
	tp.hasApi(t, func(api iface.CoreAPI) error {
		if api.Object() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestObjectAddLink", tp.TestObjectAddLink)
	t.Run("TestObjectAddLinkCreate", tp.TestObjectAddLinkCreate)
	t.Run("TestObjectRmLink", tp.TestObjectRmLink)
	t.Run("TestDiffTest", tp.TestDiffTest)
}

func putDagPbNode(t *testing.T, ctx context.Context, api iface.CoreAPI, data string, links []*ipld.Link) path.ImmutablePath {
	dagnode := new(dag.ProtoNode)

	if data != "" {
		dagnode.SetData([]byte(data))
	}

	if links != nil {
		err := dagnode.SetLinks(links)
		require.NoError(t, err)
	}

	err := api.Dag().Add(ctx, dagnode)
	require.NoError(t, err)

	return path.FromCid(dagnode.Cid())
}

func (tp *TestSuite) TestObjectAddLink(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	p1 := putDagPbNode(t, ctx, api, "foo", nil)
	p2 := putDagPbNode(t, ctx, api, "bazz", []*ipld.Link{
		{
			Name: "bar",
			Cid:  p1.RootCid(),
			Size: 3,
		},
	})

	p3, err := api.Object().AddLink(ctx, p2, "abc", p2)
	require.NoError(t, err)

	nd, err := api.Dag().Get(ctx, p3.RootCid())
	require.NoError(t, err)

	links := nd.Links()
	require.Len(t, links, 2)
	require.Equal(t, "abc", links[0].Name)
	require.Equal(t, "bar", links[1].Name)
}

func (tp *TestSuite) TestObjectAddLinkCreate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	p1 := putDagPbNode(t, ctx, api, "foo", nil)
	p2 := putDagPbNode(t, ctx, api, "bazz", []*ipld.Link{
		{
			Name: "bar",
			Cid:  p1.RootCid(),
			Size: 3,
		},
	})

	_, err = api.Object().AddLink(ctx, p2, "abc/d", p2)
	require.ErrorContains(t, err, "no link by that name")

	p3, err := api.Object().AddLink(ctx, p2, "abc/d", p2, opt.Object.Create(true))
	require.NoError(t, err)

	nd, err := api.Dag().Get(ctx, p3.RootCid())
	require.NoError(t, err)

	links := nd.Links()
	require.Len(t, links, 2)
	require.Equal(t, "abc", links[0].Name)
	require.Equal(t, "bar", links[1].Name)
}

func (tp *TestSuite) TestObjectRmLink(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	p1 := putDagPbNode(t, ctx, api, "foo", nil)
	p2 := putDagPbNode(t, ctx, api, "bazz", []*ipld.Link{
		{
			Name: "bar",
			Cid:  p1.RootCid(),
			Size: 3,
		},
	})

	p3, err := api.Object().RmLink(ctx, p2, "bar")
	require.NoError(t, err)

	nd, err := api.Dag().Get(ctx, p3.RootCid())
	require.NoError(t, err)

	links := nd.Links()
	require.Len(t, links, 0)
}

func (tp *TestSuite) TestDiffTest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	p1 := putDagPbNode(t, ctx, api, "foo", nil)
	p2 := putDagPbNode(t, ctx, api, "bar", nil)

	changes, err := api.Object().Diff(ctx, p1, p2)
	require.NoError(t, err)
	require.Len(t, changes, 1)
	require.Equal(t, iface.DiffMod, changes[0].Type)
	require.Equal(t, p1.String(), changes[0].Before.String())
	require.Equal(t, p2.String(), changes[0].After.String())
}
