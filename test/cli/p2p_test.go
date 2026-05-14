package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/ipfs/kubo/core/commands"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// waitForListenerCount waits until the node has exactly the expected number of listeners.
func waitForListenerCount(t *testing.T, node *harness.Node, expectedCount int) {
	t.Helper()
	require.Eventually(t, func() bool {
		lsOut := node.IPFS("p2p", "ls", "--enc=json")
		var lsResult commands.P2PLsOutput
		if err := json.Unmarshal(lsOut.Stdout.Bytes(), &lsResult); err != nil {
			return false
		}
		return len(lsResult.Listeners) == expectedCount
	}, 5*time.Second, 100*time.Millisecond, "expected %d listeners", expectedCount)
}

// waitForListenerProtocol waits until the node has a listener with the given protocol.
func waitForListenerProtocol(t *testing.T, node *harness.Node, protocol string) {
	t.Helper()
	require.Eventually(t, func() bool {
		lsOut := node.IPFS("p2p", "ls", "--enc=json")
		var lsResult commands.P2PLsOutput
		if err := json.Unmarshal(lsOut.Stdout.Bytes(), &lsResult); err != nil {
			return false
		}
		return slices.ContainsFunc(lsResult.Listeners, func(l commands.P2PListenerInfoOutput) bool {
			return l.Protocol == protocol
		})
	}, 5*time.Second, 100*time.Millisecond, "expected listener with protocol %s", protocol)
}

func TestP2PForeground(t *testing.T) {
	t.Parallel()

	t.Run("listen foreground creates listener and removes on interrupt", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		node.StartDaemon()

		listenPort := harness.NewRandPort()

		// Start foreground listener asynchronously
		res := node.Runner.Run(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"p2p", "listen", "--foreground", "/x/fgtest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		// Wait for listener to be created
		waitForListenerProtocol(t, node, "/x/fgtest")

		// Send SIGTERM
		_ = res.Cmd.Process.Signal(syscall.SIGTERM)
		_ = res.Cmd.Wait()

		// Wait for listener to be removed
		waitForListenerCount(t, node, 0)
	})

	t.Run("listen foreground text output on SIGTERM", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		node.StartDaemon()

		listenPort := harness.NewRandPort()

		// Run without --enc=json to test actual text output users see
		res := node.Runner.Run(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"p2p", "listen", "--foreground", "/x/sigterm", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		waitForListenerProtocol(t, node, "/x/sigterm")

		_ = res.Cmd.Process.Signal(syscall.SIGTERM)
		_ = res.Cmd.Wait()

		// Verify stdout shows "waiting for interrupt" message
		stdout := res.Stdout.String()
		require.Contains(t, stdout, "waiting for interrupt")

		// Note: "Received interrupt, removing listener" message is NOT visible to CLI on SIGTERM
		// because the command runs in the daemon via RPC and the response stream closes before
		// the message can be emitted. The important behavior is verified in the first test:
		// the listener IS removed when SIGTERM is sent.
	})

	t.Run("forward foreground creates forwarder and removes on interrupt", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		})
		nodes.StartDaemons().Connect()

		forwardPort := harness.NewRandPort()

		// Start foreground forwarder asynchronously on node 0
		res := nodes[0].Runner.Run(harness.RunRequest{
			Path:    nodes[0].IPFSBin,
			Args:    []string{"p2p", "forward", "--foreground", "/x/fgfwd", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", forwardPort), "/p2p/" + nodes[1].PeerID().String()},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		// Wait for forwarder to be created
		waitForListenerCount(t, nodes[0], 1)

		// Send SIGTERM
		_ = res.Cmd.Process.Signal(syscall.SIGTERM)
		_ = res.Cmd.Wait()

		// Wait for forwarder to be removed
		waitForListenerCount(t, nodes[0], 0)
	})

	t.Run("forward foreground text output on SIGTERM", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		})
		nodes.StartDaemons().Connect()

		forwardPort := harness.NewRandPort()

		// Run without --enc=json to test actual text output users see
		res := nodes[0].Runner.Run(harness.RunRequest{
			Path:    nodes[0].IPFSBin,
			Args:    []string{"p2p", "forward", "--foreground", "/x/fwdsigterm", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", forwardPort), "/p2p/" + nodes[1].PeerID().String()},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		waitForListenerCount(t, nodes[0], 1)

		_ = res.Cmd.Process.Signal(syscall.SIGTERM)
		_ = res.Cmd.Wait()

		// Verify stdout shows "waiting for interrupt" message
		stdout := res.Stdout.String()
		require.Contains(t, stdout, "waiting for interrupt")

		// Note: "Received interrupt, removing forwarder" message is NOT visible to CLI on SIGTERM
		// because the response stream closes before the message can be emitted.
	})

	t.Run("listen without foreground returns immediately and persists", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		node.StartDaemon()

		listenPort := harness.NewRandPort()

		// This should return immediately (not block)
		node.IPFS("p2p", "listen", "/x/nofg", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort))

		// Listener should still exist
		waitForListenerProtocol(t, node, "/x/nofg")

		// Clean up
		node.IPFS("p2p", "close", "-p", "/x/nofg")
	})

	t.Run("listen foreground text output on p2p close", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		node.StartDaemon()

		listenPort := harness.NewRandPort()

		// Run without --enc=json to test actual text output users see
		res := node.Runner.Run(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"p2p", "listen", "--foreground", "/x/closetest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		// Wait for listener to be created
		waitForListenerProtocol(t, node, "/x/closetest")

		// Close the listener via ipfs p2p close command
		node.IPFS("p2p", "close", "-p", "/x/closetest")

		// Wait for foreground command to exit (it should exit quickly after close)
		done := make(chan error, 1)
		go func() {
			done <- res.Cmd.Wait()
		}()

		select {
		case <-done:
			// Good - command exited
		case <-time.After(5 * time.Second):
			_ = res.Cmd.Process.Kill()
			t.Fatal("foreground command did not exit after listener was closed via ipfs p2p close")
		}

		// Wait for listener to be removed
		waitForListenerCount(t, node, 0)

		// Verify text output shows BOTH messages when closed via p2p close
		// (unlike SIGTERM, the stream is still open so "Received interrupt" is emitted)
		out := res.Stdout.String()
		require.Contains(t, out, "waiting for interrupt")
		require.Contains(t, out, "Received interrupt, removing listener")
	})

	t.Run("forward foreground text output on p2p close", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		})
		nodes.StartDaemons().Connect()

		forwardPort := harness.NewRandPort()

		// Run without --enc=json to test actual text output users see
		res := nodes[0].Runner.Run(harness.RunRequest{
			Path:    nodes[0].IPFSBin,
			Args:    []string{"p2p", "forward", "--foreground", "/x/fwdclose", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", forwardPort), "/p2p/" + nodes[1].PeerID().String()},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		// Wait for forwarder to be created
		waitForListenerCount(t, nodes[0], 1)

		// Close the forwarder via ipfs p2p close command
		nodes[0].IPFS("p2p", "close", "-a")

		// Wait for foreground command to exit
		done := make(chan error, 1)
		go func() {
			done <- res.Cmd.Wait()
		}()

		select {
		case <-done:
			// Good - command exited
		case <-time.After(5 * time.Second):
			_ = res.Cmd.Process.Kill()
			t.Fatal("foreground command did not exit after forwarder was closed via ipfs p2p close")
		}

		// Wait for forwarder to be removed
		waitForListenerCount(t, nodes[0], 0)

		// Verify text output shows BOTH messages when closed via p2p close
		out := res.Stdout.String()
		require.Contains(t, out, "waiting for interrupt")
		require.Contains(t, out, "Received interrupt, removing forwarder")
	})

	t.Run("listen foreground tunnel transfers data and cleans up on SIGTERM", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		})
		nodes.StartDaemons().Connect()

		httpServerPort := harness.NewRandPort()
		forwardPort := harness.NewRandPort()

		// Start HTTP server
		expectedBody := "Hello from p2p tunnel!"
		httpServer := &http.Server{
			Addr: fmt.Sprintf("127.0.0.1:%d", httpServerPort),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(expectedBody))
			}),
		}
		listener, err := net.Listen("tcp", httpServer.Addr)
		require.NoError(t, err)
		go func() { _ = httpServer.Serve(listener) }()
		defer httpServer.Close()

		// Node 0: listen --foreground
		listenRes := nodes[0].Runner.Run(harness.RunRequest{
			Path:    nodes[0].IPFSBin,
			Args:    []string{"p2p", "listen", "--foreground", "/x/httptest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", httpServerPort)},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, listenRes.Err)

		// Wait for listener to be created
		waitForListenerProtocol(t, nodes[0], "/x/httptest")

		// Node 1: forward (non-foreground)
		nodes[1].IPFS("p2p", "forward", "/x/httptest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", forwardPort), "/p2p/"+nodes[0].PeerID().String())

		// Verify data flows through tunnel
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", forwardPort))
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, expectedBody, string(body))

		// Clean up forwarder on node 1
		nodes[1].IPFS("p2p", "close", "-a")

		// SIGTERM the listen --foreground command
		_ = listenRes.Cmd.Process.Signal(syscall.SIGTERM)
		_ = listenRes.Cmd.Wait()

		// Wait for listener to be removed on node 0
		waitForListenerCount(t, nodes[0], 0)
	})

	t.Run("forward foreground tunnel transfers data and cleans up on SIGTERM", func(t *testing.T) {
		t.Parallel()
		nodes := harness.NewT(t).NewNodes(2).Init()
		nodes.ForEachPar(func(n *harness.Node) {
			n.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		})
		nodes.StartDaemons().Connect()

		httpServerPort := harness.NewRandPort()
		forwardPort := harness.NewRandPort()

		// Start HTTP server
		expectedBody := "Hello from forward foreground tunnel!"
		httpServer := &http.Server{
			Addr: fmt.Sprintf("127.0.0.1:%d", httpServerPort),
			Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(expectedBody))
			}),
		}
		listener, err := net.Listen("tcp", httpServer.Addr)
		require.NoError(t, err)
		go func() { _ = httpServer.Serve(listener) }()
		defer httpServer.Close()

		// Node 0: listen (non-foreground)
		nodes[0].IPFS("p2p", "listen", "/x/httptest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", httpServerPort))

		// Node 1: forward --foreground
		forwardRes := nodes[1].Runner.Run(harness.RunRequest{
			Path:    nodes[1].IPFSBin,
			Args:    []string{"p2p", "forward", "--foreground", "/x/httptest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", forwardPort), "/p2p/" + nodes[0].PeerID().String()},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, forwardRes.Err)

		// Wait for forwarder to be created
		waitForListenerCount(t, nodes[1], 1)

		// Verify data flows through tunnel
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", forwardPort))
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, expectedBody, string(body))

		// SIGTERM the forward --foreground command
		_ = forwardRes.Cmd.Process.Signal(syscall.SIGTERM)
		_ = forwardRes.Cmd.Wait()

		// Wait for forwarder to be removed on node 1
		waitForListenerCount(t, nodes[1], 0)

		// Clean up listener on node 0
		nodes[0].IPFS("p2p", "close", "-a")
	})

	t.Run("foreground command exits when daemon shuts down", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("config", "--json", "Experimental.Libp2pStreamMounting", "true")
		node.StartDaemon()

		listenPort := harness.NewRandPort()

		// Start foreground listener
		res := node.Runner.Run(harness.RunRequest{
			Path:    node.IPFSBin,
			Args:    []string{"p2p", "listen", "--foreground", "/x/daemontest", fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
			RunFunc: (*exec.Cmd).Start,
		})
		require.NoError(t, res.Err)

		// Wait for listener to be created
		waitForListenerProtocol(t, node, "/x/daemontest")

		// Stop the daemon
		node.StopDaemon()

		// Wait for foreground command to exit
		done := make(chan error, 1)
		go func() {
			done <- res.Cmd.Wait()
		}()

		select {
		case <-done:
			// Good - foreground command exited when daemon stopped
		case <-time.After(5 * time.Second):
			_ = res.Cmd.Process.Kill()
			t.Fatal("foreground command did not exit when daemon was stopped")
		}
	})
}
