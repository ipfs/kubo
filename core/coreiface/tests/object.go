package tests

import (
	"context"
	"testing"

	dag "github.com/ipfs/boxo/ipld/merkledag"
	ft "github.com/ipfs/boxo/ipld/unixfs"
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
	t.Run("TestObjectAddLinkValidation", tp.TestObjectAddLinkValidation)
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
	ctx := t.Context()
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

	// Raw dag-pb nodes require SkipUnixFSValidation since they have no UnixFS metadata
	p3, err := api.Object().AddLink(ctx, p2, "abc", p2, opt.Object.SkipUnixFSValidation(true))
	require.NoError(t, err)

	nd, err := api.Dag().Get(ctx, p3.RootCid())
	require.NoError(t, err)

	links := nd.Links()
	require.Len(t, links, 2)
	require.Equal(t, "abc", links[0].Name)
	require.Equal(t, "bar", links[1].Name)
}

func (tp *TestSuite) TestObjectAddLinkCreate(t *testing.T) {
	ctx := t.Context()
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

	// Raw dag-pb nodes require SkipUnixFSValidation since they have no UnixFS metadata
	_, err = api.Object().AddLink(ctx, p2, "abc/d", p2, opt.Object.SkipUnixFSValidation(true))
	require.ErrorContains(t, err, "no link by that name")

	p3, err := api.Object().AddLink(ctx, p2, "abc/d", p2, opt.Object.Create(true), opt.Object.SkipUnixFSValidation(true))
	require.NoError(t, err)

	nd, err := api.Dag().Get(ctx, p3.RootCid())
	require.NoError(t, err)

	links := nd.Links()
	require.Len(t, links, 2)
	require.Equal(t, "abc", links[0].Name)
	require.Equal(t, "bar", links[1].Name)
}

// TestObjectAddLinkValidation verifies that AddLink rejects non-directory
// nodes by default, preventing the data-loss bug in
// https://github.com/ipfs/kubo/issues/7190
func (tp *TestSuite) TestObjectAddLinkValidation(t *testing.T) {
	ctx := t.Context()
	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	child := putDagPbNode(t, ctx, api, "child", nil)

	// UnixFS Directory: allowed
	dirNode := ft.EmptyDirNode()
	err = api.Dag().Add(ctx, dirNode)
	require.NoError(t, err)
	dirPath := path.FromCid(dirNode.Cid())

	_, err = api.Object().AddLink(ctx, dirPath, "foo", child)
	require.NoError(t, err)

	// UnixFS File: rejected (would cause data loss on read-back)
	fileNode := ft.EmptyFileNode()
	err = api.Dag().Add(ctx, fileNode)
	require.NoError(t, err)
	filePath := path.FromCid(fileNode.Cid())

	_, err = api.Object().AddLink(ctx, filePath, "foo", child)
	require.ErrorContains(t, err, "cannot add named links to a UnixFS File node, only Directory nodes support link addition at the dag-pb level")

	// UnixFS File with SkipUnixFSValidation: allowed (user takes responsibility)
	_, err = api.Object().AddLink(ctx, filePath, "foo", child, opt.Object.SkipUnixFSValidation(true))
	require.NoError(t, err)

	// HAMTShard: rejected (dag-pb level mutation corrupts HAMT bitfield)
	hamtData, err := ft.HAMTShardData(nil, 256, 0x22)
	require.NoError(t, err)
	hamtNode := new(dag.ProtoNode)
	hamtNode.SetData(hamtData)
	err = api.Dag().Add(ctx, hamtNode)
	require.NoError(t, err)
	hamtPath := path.FromCid(hamtNode.Cid())

	_, err = api.Object().AddLink(ctx, hamtPath, "foo", child)
	require.ErrorContains(t, err, "cannot add links to a HAMTShard at the dag-pb level (would corrupt the HAMT bitfield); use 'ipfs files' commands instead, or pass --allow-non-unixfs to override")

	// HAMTShard with SkipUnixFSValidation: allowed
	_, err = api.Object().AddLink(ctx, hamtPath, "foo", child, opt.Object.SkipUnixFSValidation(true))
	require.NoError(t, err)

	// Raw dag-pb (no UnixFS data): rejected
	rawPb := putDagPbNode(t, ctx, api, "", nil)

	_, err = api.Object().AddLink(ctx, rawPb, "foo", child)
	require.ErrorContains(t, err, "cannot add named links to a non-UnixFS dag-pb node; pass --allow-non-unixfs to skip validation")

	// Raw dag-pb with SkipUnixFSValidation: allowed
	_, err = api.Object().AddLink(ctx, rawPb, "foo", child, opt.Object.SkipUnixFSValidation(true))
	require.NoError(t, err)
}

func (tp *TestSuite) TestObjectRmLink(t *testing.T) {
	ctx := t.Context()
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
	ctx := t.Context()
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
