package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/guseggert/go-ds-grpc/proto"
	"github.com/guseggert/go-ds-grpc/server"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const grpcDatastoreSpec = `
{
    "type": "grpcds",
    "name": "grpc-datastore",
    "target": "%s",
    "allowInsecure": true
}
`

func TestGRPCDatastore(t *testing.T) {
	// we init the node to get the default config, then modify it, then re-init the node
	node := harness.NewT(t).NewNode().Init()

	// run grpc datastore server
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	dsServer := server.New(ds)
	grpcServer := grpc.NewServer()
	pb.RegisterDatastoreServer(grpcServer, dsServer)

	l, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	go func() {
		if err := grpcServer.Serve(l); err != nil {
			t.Logf("grpc server error: %s", err)
		}
	}()
	defer grpcServer.Stop()

	// update the config
	spec := fmt.Sprintf(grpcDatastoreSpec, l.Addr().String())
	fmt.Printf("using spec: \n%s\n", spec)
	specMap := map[string]interface{}{}
	err = json.Unmarshal([]byte(spec), &specMap)
	require.NoError(t, err)
	node.UpdateConfig(func(cfg *config.Config) {
		cfg.Datastore.Spec = specMap
	})

	// copy config to a new file and re-init the repo to initialize the datastore
	config := node.ReadFile(node.ConfigFile())
	require.NoError(t, os.RemoveAll(node.Dir))
	require.NoError(t, os.Mkdir(node.Dir, 0777))
	node.WriteBytes("config-backup", []byte(config))
	node.IPFS("init", filepath.Join(node.Dir, "config-backup"))

	node.StartDaemon()

	randStr := string(testutils.RandomBytes(100))
	node.IPFSAddStr(randStr)

	// verify the datastore has stuff in it
	keys := map[string]bool{}
	results, err := ds.Query(context.Background(), query.Query{})
	require.NoError(t, err)
	for res := range results.Next() {
		keys[res.Entry.Key] = true
	}
	assert.True(t, keys["/pins/state/dirty"])

	// TODO ensure daemon won't launch when grpc server is not running
}
