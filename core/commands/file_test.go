package commands

import (
	"context"
	"io"
	"strings"
	"testing"

	cmds "github.com/ipfs/go-ipfs-cmds"
	coremock "github.com/ipfs/kubo/core/mock"
)

// type testenv struct {
// 	ctx     context.Context
// 	node    *core.IpfsNode
// 	reqchan chan<- string
// }

// type writeCloser struct {
// 	*bytes.Buffer
// }

// func (w writeCloser) Close() error { return nil }

// func createTestEnv(t *testing.T, ctx context.Context) cmds.Environment {
// 	ds := sync.MutexWrap(datastore.NewMapDatastore())
// 	node, err := core.NewNode(ctx, &core.BuildCfg{
// 		Online: false,
// 		Repo: &repo.Mock{
// 			C: config.Config{
// 				Identity: config.Identity{
// 					PeerID: "QmTFauExutTsy4XP6JbMFcw2Wa9645HJt2bTqL6qYDCKfe",
// 				},
// 			},
// 			D: ds,
// 		},
// 	})
// 	require.NoError(t, err)

// 	return &testenv{
// 		ctx:  ctx,
// 		node: node,
// 	}

// }

// func TestCopyCBORtoMFS(t *testing.T) {
// 	// mock environment creation
// 	ctx := context.Background()
// 	env := createTestEnv(t, ctx)

// 	cborCid := "bafyreigbtj4x7ip5legnfznufuopl4sg4knzc2cof6duas4b3q2fy6swua"

// 	req := &cmds.Request{
// 		Context: ctx,
// 		Arguments: []string{
// 			"/ipfs/" + cborCid,
// 			"/test-cbor",
// 		},
// 		Options: map[string]interface{}{
// 			filesFlushOptionName: true,
// 		},
// 	}

// 	// mock response emitter
// 	w := writeCloser{new(bytes.Buffer)}
// 	require.NoError(t, err, "creating response emitter should not fail")

// 	err = filesCpCmd.Run(req, res, env)
// 	if err != nil {
// 		t.Logf("Error during file copy: %v", err) // Print actual error
// 		fmt.Println("Error:", err)                // Alternative direct print
// 	}

// 	require.Error(t, err, "copying dag-cbor should fail")
// 	require.Contains(t, err.Error(), "must be a UnixFS node or raw data")
// 	// return fmt.Errorf("cp: source must be a UnixFS node or raw data")
// }

func TestFilesCp_DagCborNodeFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// _, err := coremock.NewMockNode()
	// if err != nil {
	// 	t.Fatal(err)
	// }

	cmdCtx, err := coremock.MockCmdsCtx()
	if err != nil {
		t.Fatal(err)
	}

	// env := cmdenv.Environment{
	// 	Node:    node,
	// 	CoreAPI: coreapi.NewCoreAPI(node),
	// }

	cborCID := "bafyreigbtj4x7ip5legnfznufuopl4sg4knzc2cof6duas4b3q2fy6swua"
	req := &cmds.Request{
		Context: ctx,
		Arguments: []string{
			"/ipfs/" + cborCID,
			"/test-destination",
		},
		Options: map[string]interface{}{
			"force": false,
		},
	}

	_, pw := io.Pipe()
	res, err := cmds.NewWriterResponseEmitter(pw, req)
	if err != nil {
		t.Fatal(err)
	}
	err = filesCpCmd.Run(req, res, &cmdCtx)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	expectedErr := "cp: source must be a UnixFS node or raw data"
	if !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("got error %q, want %q", err.Error(), expectedErr)
	}
}
