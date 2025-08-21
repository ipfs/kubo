package cli

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/go-test/random"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils/httprouting"
	"github.com/stretchr/testify/assert"
)

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
	mockRouter := &httprouting.MockHTTPContentRouter{}

	// run the content routing HTTP server
	userAgentRecorder := &userAgentRecorder{delegate: server.Handler(mockRouter)}
	server := httptest.NewServer(userAgentRecorder)
	t.Cleanup(func() { server.Close() })

	// setup the node
	node := harness.NewT(t).NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		// setup Kubo node to use mocked HTTP Router
		cfg.Routing.DelegatedRouters = []string{server.URL}
	})
	node.StartDaemon()

	// compute a random CID
	randStr := string(random.Bytes(100))
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
			return mockRouter.NumFindProvidersCalls() > 0
		}, time.Minute, 10*time.Millisecond)

		assert.NotEmpty(t, userAgentRecorder.userAgents)
		version := node.IPFS("id", "-f", "<aver>").Stdout.Trimmed()
		for _, userAgent := range userAgentRecorder.userAgents {
			assert.Equal(t, version, userAgent)
		}
	})
}
