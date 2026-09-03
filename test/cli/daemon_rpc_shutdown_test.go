package cli

import (
	"os"
	"testing"
	"time"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// rpcShutdownExitBound is how long the daemon process may take to exit after
// "ipfs shutdown" returns. A clean shutdown finishes in a few seconds; hitting
// this bound means the daemon wedged after closing the node.
const rpcShutdownExitBound = 15 * time.Second

// TestDaemonRPCShutdown covers supervisors that stop the daemon over the RPC
// API ("ipfs shutdown") instead of sending a signal. The process must exit on
// its own. The harness StopDaemon sends SIGTERM, which would hide a hang, so
// the assertion waits on the process directly.
func TestDaemonRPCShutdown(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "without gc", args: nil},
		{name: "with --enable-gc", args: []string{"--enable-gc"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init()
			node.StartDaemon(tc.args...)

			type waitResult struct {
				state *os.ProcessState
				err   error
			}
			exited := make(chan waitResult, 1)
			go func() {
				state, err := node.Daemon.Cmd.Process.Wait()
				exited <- waitResult{state, err}
			}()

			node.IPFS("shutdown")

			select {
			case res := <-exited:
				require.NoError(t, res.err)
				require.True(t, res.state.Success(), "daemon exited with %s", res.state)
			case <-time.After(rpcShutdownExitBound):
				// Kill so the harness cleanup does not wait on a wedged daemon.
				_ = node.Daemon.Cmd.Process.Kill()
				t.Fatalf("daemon did not exit within %s after RPC shutdown", rpcShutdownExitBound)
			}
		})
	}
}
