package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/boxo/routing/http/types/iter"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/stretchr/testify/assert"
)

type fakeHTTPContentRouter struct {
	m                  sync.Mutex
	findProvidersCalls int
	provideCalls       int
}

func (r *fakeHTTPContentRouter) FindProviders(ctx context.Context, key cid.Cid, limit int) (iter.ResultIter[types.ProviderResponse], error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.findProvidersCalls++
	return iter.FromSlice([]iter.Result[types.ProviderResponse]{}), nil
}

func (r *fakeHTTPContentRouter) ProvideBitswap(ctx context.Context, req *server.BitswapWriteProvideRequest) (time.Duration, error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.provideCalls++
	return 0, nil
}
func (r *fakeHTTPContentRouter) Provide(ctx context.Context, req *server.WriteProvideRequest) (types.ProviderResponse, error) {
	r.m.Lock()
	defer r.m.Unlock()
	r.provideCalls++
	return nil, nil
}

func (r *fakeHTTPContentRouter) numFindProvidersCalls() int {
	r.m.Lock()
	defer r.m.Unlock()
	return r.findProvidersCalls
}

// userAgentRecorder records the user agent of every HTTP request
type userAgentRecorder struct {
	delegate   http.Handler
	userAgents []string
}

func (r *userAgentRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.userAgents = append(r.userAgents, req.UserAgent())
	r.delegate.ServeHTTP(w, req)
}

func TestContentRoutingHTTP(t *testing.T) {
	cr := &fakeHTTPContentRouter{}

	// run the content routing HTTP server
	userAgentRecorder := &userAgentRecorder{delegate: server.Handler(cr)}
	server := httptest.NewServer(userAgentRecorder)
	t.Cleanup(func() { server.Close() })

	// setup the node
	node := harness.NewT(t).NewNode().Init()
	node.Runner.Env["IPFS_HTTP_ROUTERS"] = server.URL
	node.StartDaemon()

	// compute a random CID
	randStr := string(testutils.RandomBytes(100))
	res := node.PipeStrToIPFS(randStr, "add", "-qn")
	wantCIDStr := res.Stdout.Trimmed()

	t.Run("fetching an uncached block results in an HTTP lookup", func(t *testing.T) {
		statRes := node.Runner.Run(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"block", "stat", wantCIDStr},
			RunFunc: (*exec.Cmd).Start,
		})
		defer func() {
			if err := statRes.Cmd.Process.Kill(); err != nil {
				t.Logf("error killing 'block stat' cmd: %s", err)
			}
		}()

		// verify the content router was called
		assert.Eventually(t, func() bool {
			return cr.numFindProvidersCalls() > 0
		}, time.Minute, 10*time.Millisecond)

		assert.NotEmpty(t, userAgentRecorder.userAgents)
		version := node.IPFS("id", "-f", "<aver>").Stdout.Trimmed()
		for _, userAgent := range userAgentRecorder.userAgents {
			assert.Equal(t, version, userAgent)
		}
	})
}
