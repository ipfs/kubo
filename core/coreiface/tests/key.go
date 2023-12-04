package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/ipfs/boxo/ipns"
	"github.com/ipfs/go-cid"
	iface "github.com/ipfs/kubo/core/coreiface"
	opt "github.com/ipfs/kubo/core/coreiface/options"
	"github.com/libp2p/go-libp2p/core/peer"
	mbase "github.com/multiformats/go-multibase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (tp *TestSuite) TestKey(t *testing.T) {
	tp.hasApi(t, func(api iface.CoreAPI) error {
		if api.Key() == nil {
			return errAPINotImplemented
		}
		return nil
	})

	t.Run("TestListSelf", tp.TestListSelf)
	t.Run("TestRenameSelf", tp.TestRenameSelf)
	t.Run("TestRemoveSelf", tp.TestRemoveSelf)
	t.Run("TestGenerate", tp.TestGenerate)
	t.Run("TestGenerateSize", tp.TestGenerateSize)
	t.Run("TestGenerateType", tp.TestGenerateType)
	t.Run("TestGenerateExisting", tp.TestGenerateExisting)
	t.Run("TestList", tp.TestList)
	t.Run("TestRename", tp.TestRename)
	t.Run("TestRenameToSelf", tp.TestRenameToSelf)
	t.Run("TestRenameToSelfForce", tp.TestRenameToSelfForce)
	t.Run("TestRenameOverwriteNoForce", tp.TestRenameOverwriteNoForce)
	t.Run("TestRenameOverwrite", tp.TestRenameOverwrite)
	t.Run("TestRenameSameNameNoForce", tp.TestRenameSameNameNoForce)
	t.Run("TestRenameSameName", tp.TestRenameSameName)
	t.Run("TestSign", tp.TestSign)
	t.Run("TestVerify", tp.TestVerify)
}

func (tp *TestSuite) TestListSelf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	self, err := api.Key().Self(ctx)
	require.NoError(t, err)

	keys, err := api.Key().List(ctx)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, "self", keys[0].Name())
	assert.Equal(t, "/ipns/"+iface.FormatKeyID(self.ID()), keys[0].Path().String())
}

func (tp *TestSuite) TestRenameSelf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, _, err = api.Key().Rename(ctx, "self", "foo")
	require.ErrorContains(t, err, "cannot rename key with name 'self'")

	_, _, err = api.Key().Rename(ctx, "self", "foo", opt.Key.Force(true))
	require.ErrorContains(t, err, "cannot rename key with name 'self'")
}

func (tp *TestSuite) TestRemoveSelf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Remove(ctx, "self")
	require.ErrorContains(t, err, "cannot remove key with name 'self'")
}

func (tp *TestSuite) TestGenerate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	k, err := api.Key().Generate(ctx, "foo")
	require.NoError(t, err)
	require.Equal(t, "foo", k.Name())

	verifyIPNSPath(t, k.Path().String())
}

func verifyIPNSPath(t *testing.T, p string) {
	t.Helper()

	require.True(t, strings.HasPrefix(p, "/ipns/"))

	k := p[len("/ipns/"):]
	c, err := cid.Decode(k)
	require.NoError(t, err)

	b36, err := c.StringOfBase(mbase.Base36)
	require.NoError(t, err)
	require.Equal(t, k, b36)
}

func (tp *TestSuite) TestGenerateSize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	k, err := api.Key().Generate(ctx, "foo", opt.Key.Size(2048))
	require.NoError(t, err)
	require.Equal(t, "foo", k.Name())

	verifyIPNSPath(t, k.Path().String())
}

func (tp *TestSuite) TestGenerateType(t *testing.T) {
	t.Skip("disabled until libp2p/specs#111 is fixed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	k, err := api.Key().Generate(ctx, "bar", opt.Key.Type(opt.Ed25519Key))
	require.NoError(t, err)
	require.Equal(t, "bar", k.Name())
	// Expected to be an inlined identity hash.
	require.True(t, strings.HasPrefix(k.Path().String(), "/ipns/12"))
}

func (tp *TestSuite) TestGenerateExisting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.ErrorContains(t, err, "key with name 'foo' already exists")

	_, err = api.Key().Generate(ctx, "self")
	require.ErrorContains(t, err, "cannot create key with name 'self'")
}

func (tp *TestSuite) TestList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	l, err := api.Key().List(ctx)
	require.NoError(t, err)
	require.Len(t, l, 2)
	require.Equal(t, "self", l[0].Name())
	require.Equal(t, "foo", l[1].Name())

	verifyIPNSPath(t, l[0].Path().String())
	verifyIPNSPath(t, l[1].Path().String())
}

func (tp *TestSuite) TestRename(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	k, overwrote, err := api.Key().Rename(ctx, "foo", "bar")
	require.NoError(t, err)
	assert.False(t, overwrote)
	assert.Equal(t, "bar", k.Name())
}

func (tp *TestSuite) TestRenameToSelf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	_, _, err = api.Key().Rename(ctx, "foo", "self")
	require.ErrorContains(t, err, "cannot overwrite key with name 'self'")
}

func (tp *TestSuite) TestRenameToSelfForce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	_, _, err = api.Key().Rename(ctx, "foo", "self", opt.Key.Force(true))
	require.ErrorContains(t, err, "cannot overwrite key with name 'self'")
}

func (tp *TestSuite) TestRenameOverwriteNoForce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "bar")
	require.NoError(t, err)

	_, _, err = api.Key().Rename(ctx, "foo", "bar")
	require.ErrorContains(t, err, "key by that name already exists, refusing to overwrite")
}

func (tp *TestSuite) TestRenameOverwrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	kfoo, err := api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "bar")
	require.NoError(t, err)

	k, overwrote, err := api.Key().Rename(ctx, "foo", "bar", opt.Key.Force(true))
	require.NoError(t, err)
	require.True(t, overwrote)
	assert.Equal(t, "bar", k.Name())
	assert.Equal(t, kfoo.Path().String(), k.Path().String())
}

func (tp *TestSuite) TestRenameSameNameNoForce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	k, overwrote, err := api.Key().Rename(ctx, "foo", "foo")
	require.NoError(t, err)
	assert.False(t, overwrote)
	assert.Equal(t, "foo", k.Name())
}

func (tp *TestSuite) TestRenameSameName(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	_, err = api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	k, overwrote, err := api.Key().Rename(ctx, "foo", "foo", opt.Key.Force(true))
	require.NoError(t, err)
	assert.False(t, overwrote)
	assert.Equal(t, "foo", k.Name())
}

func (tp *TestSuite) TestRemove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	k, err := api.Key().Generate(ctx, "foo")
	require.NoError(t, err)

	l, err := api.Key().List(ctx)
	require.NoError(t, err)
	require.Len(t, l, 2)

	p, err := api.Key().Remove(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, p.Path().String(), k.Path().String())

	l, err = api.Key().List(ctx)
	require.NoError(t, err)
	require.Len(t, l, 1)
	assert.Equal(t, "self", l[0].Name())
}

func (tp *TestSuite) TestSign(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api, err := tp.makeAPI(t, ctx)
	require.NoError(t, err)

	key1, err := api.Key().Generate(ctx, "foo", opt.Key.Type(opt.Ed25519Key))
	require.NoError(t, err)

	data := []byte("hello world")

	key2, signature, err := api.Key().Sign(ctx, "foo", data)
	require.NoError(t, err)

	require.Equal(t, key1.Name(), key2.Name())
	require.Equal(t, key1.ID(), key2.ID())

	pk, err := key1.ID().ExtractPublicKey()
	require.NoError(t, err)

	valid, err := pk.Verify(append([]byte("libp2p-key signed message:"), data...), signature)
	require.NoError(t, err)
	require.True(t, valid)
}

func (tp *TestSuite) TestVerify(t *testing.T) {
	t.Parallel()

	t.Run("Verify Own Key", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		api, err := tp.makeAPI(t, ctx)
		require.NoError(t, err)

		_, err = api.Key().Generate(ctx, "foo", opt.Key.Type(opt.Ed25519Key))
		require.NoError(t, err)

		data := []byte("hello world")

		_, signature, err := api.Key().Sign(ctx, "foo", data)
		require.NoError(t, err)

		_, valid, err := api.Key().Verify(ctx, "foo", signature, data)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("Verify Self", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		api, err := tp.makeAPIWithIdentityAndOffline(t, ctx)
		require.NoError(t, err)

		data := []byte("hello world")

		_, signature, err := api.Key().Sign(ctx, "", data)
		require.NoError(t, err)

		_, valid, err := api.Key().Verify(ctx, "", signature, data)
		require.NoError(t, err)
		require.True(t, valid)
	})

	t.Run("Verify With Key In Different Formats", func(t *testing.T) {
		t.Parallel()

		// Spin some node and get signature out.
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		api, err := tp.makeAPI(t, ctx)
		require.NoError(t, err)

		key, err := api.Key().Generate(ctx, "foo", opt.Key.Type(opt.Ed25519Key))
		require.NoError(t, err)

		data := []byte("hello world")

		_, signature, err := api.Key().Sign(ctx, "foo", data)
		require.NoError(t, err)

		for _, testCase := range [][]string{
			{"Base58 Encoded Peer ID", key.ID().String()},
			{"CIDv1 Encoded Peer ID", peer.ToCid(key.ID()).String()},
			{"CIDv1 Encoded IPNS Name", ipns.NameFromPeer(key.ID()).String()},
			{"Prefixed IPNS Path", ipns.NameFromPeer(key.ID()).AsPath().String()},
		} {
			t.Run(testCase[0], func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// Spin new node.
				api, err := tp.makeAPI(t, ctx)
				require.NoError(t, err)

				_, valid, err := api.Key().Verify(ctx, testCase[1], signature, data)
				require.NoError(t, err)
				require.True(t, valid)
			})
		}
	})
}
