package cli

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils"
	"github.com/ipfs/kubo/test/cli/testutils/pinningservice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func runPinningService(t *testing.T, authToken string) (*pinningservice.PinningService, string) {
	svc := pinningservice.New()
	router := pinningservice.NewRouter(authToken, svc)
	server := &http.Server{Handler: router}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, net.ErrClosed) && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Serve error: %s", err)
		}
	}()
	t.Cleanup(func() { listener.Close() })

	return svc, fmt.Sprintf("http://%s/api/v1", listener.Addr().String())
}

func TestRemotePinning(t *testing.T) {
	t.Parallel()
	authToken := "testauthtoken"

	t.Run("MFS pinning", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.Runner.Env["MFS_PIN_POLL_INTERVAL"] = "10ms"

		_, svcURL := runPinningService(t, authToken)
		node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)
		node.IPFS("config", "--json", "Pinning.RemoteServices.svc.Policies.MFS.RepinInterval", `"1s"`)
		node.IPFS("config", "--json", "Pinning.RemoteServices.svc.Policies.MFS.PinName", `"test_pin"`)
		node.IPFS("config", "--json", "Pinning.RemoteServices.svc.Policies.MFS.Enable", "true")

		node.StartDaemon()

		node.IPFS("files", "cp", "/ipfs/bafkqaaa", "/mfs-pinning-test-"+uuid.NewString())
		node.IPFS("files", "flush")
		res := node.IPFS("files", "stat", "/", "--enc=json")
		hash := gjson.Get(res.Stdout.String(), "Hash").Str

		assert.Eventually(t,
			func() bool {
				res = node.IPFS("pin", "remote", "ls",
					"--service=svc",
					"--name=test_pin",
					"--status=queued,pinning,pinned,failed",
					"--enc=json",
				)
				pinnedHash := gjson.Get(res.Stdout.String(), "Cid").Str
				return hash == pinnedHash
			},
			10*time.Second,
			10*time.Millisecond,
		)

		t.Run("MFS root is repinned on CID change", func(t *testing.T) {
			node.IPFS("files", "cp", "/ipfs/bafkqaaa", "/mfs-pinning-repin-test-"+uuid.NewString())
			node.IPFS("files", "flush")
			res = node.IPFS("files", "stat", "/", "--enc=json")
			hash := gjson.Get(res.Stdout.String(), "Hash").Str
			assert.Eventually(t,
				func() bool {
					res := node.IPFS("pin", "remote", "ls",
						"--service=svc",
						"--name=test_pin",
						"--status=queued,pinning,pinned,failed",
						"--enc=json",
					)
					pinnedHash := gjson.Get(res.Stdout.String(), "Cid").Str
					return hash == pinnedHash
				},
				10*time.Second,
				10*time.Millisecond,
			)
		})
	})

	// Pinning.RemoteServices includes API.Key, so we give it the same treatment
	// as Identity,PrivKey to prevent exposing it on the network
	t.Run("access token security", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		node.IPFS("pin", "remote", "service", "add", "1", "http://example1.com", "testkey")
		res := node.RunIPFS("config", "Pinning")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "cannot show or change pinning services credentials")
		assert.NotContains(t, res.Stdout.String(), "testkey")

		res = node.RunIPFS("config", "Pinning.RemoteServices.1.API.Key")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "cannot show or change pinning services credentials")
		assert.NotContains(t, res.Stdout.String(), "testkey")

		configShow := node.RunIPFS("config", "show").Stdout.String()
		assert.NotContains(t, configShow, "testkey")

		t.Run("re-injecting config with 'ipfs config replace' preserves the API keys", func(t *testing.T) {
			node.WriteBytes("config-show", []byte(configShow))
			node.IPFS("config", "replace", "config-show")
			assert.Contains(t, node.ReadFile(node.ConfigFile()), "testkey")
		})

		t.Run("injecting config with 'ipfs config replace' with API keys returns an error", func(t *testing.T) {
			// remove Identity.PrivKey to ensure error is triggered by Pinning.RemoteServices
			configJSON := MustVal(sjson.Delete(configShow, "Identity.PrivKey"))
			configJSON = MustVal(sjson.Set(configJSON, "Pinning.RemoteServices.1.API.Key", "testkey"))
			node.WriteBytes("new-config", []byte(configJSON))
			res := node.RunIPFS("config", "replace", "new-config")
			assert.Equal(t, 1, res.ExitCode())
			assert.Contains(t, res.Stderr.String(), "cannot change remote pinning services api info with `config replace`")
		})
	})

	t.Run("pin remote service ls --stat", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		_, svcURL := runPinningService(t, authToken)

		node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)
		node.IPFS("pin", "remote", "service", "add", "invalid-svc", svcURL+"/invalidpath", authToken)

		res := node.IPFS("pin", "remote", "service", "ls", "--stat")
		assert.Contains(t, res.Stdout.String(), " 0/0/0/0")

		stats := node.IPFS("pin", "remote", "service", "ls", "--stat", "--enc=json").Stdout.String()
		assert.Equal(t, "valid", gjson.Get(stats, `RemoteServices.#(Service == "svc").Stat.Status`).Str)
		assert.Equal(t, "invalid", gjson.Get(stats, `RemoteServices.#(Service == "invalid-svc").Stat.Status`).Str)

		// no --stat returns no stat obj
		t.Run("no --stat returns no stat obj", func(t *testing.T) {
			res := node.IPFS("pin", "remote", "service", "ls", "--enc=json")
			assert.False(t, gjson.Get(res.Stdout.String(), `RemoteServices.#(Service == "svc").Stat`).Exists())
		})
	})

	t.Run("adding service with invalid URL fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()

		res := node.RunIPFS("pin", "remote", "service", "add", "svc", "invalid-service.example.com", "key")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "service endpoint must be a valid HTTP URL")

		res = node.RunIPFS("pin", "remote", "service", "add", "svc", "xyz://invalid-service.example.com", "key")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "service endpoint must be a valid HTTP URL")
	})

	t.Run("unauthorized pinning service calls fail", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		_, svcURL := runPinningService(t, authToken)

		node.IPFS("pin", "remote", "service", "add", "svc", svcURL, "othertoken")

		res := node.RunIPFS("pin", "remote", "ls", "--service=svc")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "access denied")
	})

	t.Run("pinning service calls fail when there is a wrong path", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		_, svcURL := runPinningService(t, authToken)
		node.IPFS("pin", "remote", "service", "add", "svc", svcURL+"/invalid-path", authToken)

		res := node.RunIPFS("pin", "remote", "ls", "--service=svc")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "404")
	})

	t.Run("pinning service calls fail when DNS resolution fails", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		node.IPFS("pin", "remote", "service", "add", "svc", "https://invalid-service.example.com", authToken)

		res := node.RunIPFS("pin", "remote", "ls", "--service=svc")
		assert.Equal(t, 1, res.ExitCode())
		assert.Contains(t, res.Stderr.String(), "no such host")
	})

	t.Run("pin remote service rm", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init().StartDaemon()
		node.IPFS("pin", "remote", "service", "add", "svc", "https://example.com", authToken)
		node.IPFS("pin", "remote", "service", "rm", "svc")
		res := node.IPFS("pin", "remote", "service", "ls")
		assert.NotContains(t, res.Stdout.String(), "svc")
	})

	t.Run("remote pinning", func(t *testing.T) {
		t.Parallel()

		verifyStatus := func(node *harness.Node, name, hash, status string) {
			resJSON := node.IPFS("pin", "remote", "ls",
				"--service=svc",
				"--enc=json",
				"--name="+name,
				"--status="+status,
			).Stdout.String()

			assert.Equal(t, status, gjson.Get(resJSON, "Status").Str)
			assert.Equal(t, hash, gjson.Get(resJSON, "Cid").Str)
			assert.Equal(t, name, gjson.Get(resJSON, "Name").Str)
		}

		t.Run("'ipfs pin remote add --background=true'", func(t *testing.T) {
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			// retain a ptr to the pin that's in the DB so we can directly mutate its status
			// to simulate async work
			pinCh := make(chan *pinningservice.PinStatus, 1)
			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pinCh <- pin
			}

			hash := node.IPFSAddStr("foo")
			node.IPFS("pin", "remote", "add",
				"--background=true",
				"--service=svc",
				"--name=pin1",
				hash,
			)

			pin := <-pinCh

			transitionStatus := func(status string) {
				pin.M.Lock()
				pin.Status = status
				pin.M.Unlock()
			}

			verifyStatus(node, "pin1", hash, "queued")

			transitionStatus("pinning")
			verifyStatus(node, "pin1", hash, "pinning")

			transitionStatus("pinned")
			verifyStatus(node, "pin1", hash, "pinned")

			transitionStatus("failed")
			verifyStatus(node, "pin1", hash, "failed")
		})

		t.Run("'ipfs pin remote add --background=false'", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pin.Status = "pinned"
			}
			hash := node.IPFSAddStr("foo")
			node.IPFS("pin", "remote", "add",
				"--background=false",
				"--service=svc",
				"--name=pin2",
				hash,
			)
			verifyStatus(node, "pin2", hash, "pinned")
		})

		t.Run("'ipfs pin remote ls' with multiple statuses", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			hash := node.IPFSAddStr("foo")
			desiredStatuses := map[string]string{
				"pin-queued":  "queued",
				"pin-pinning": "pinning",
				"pin-pinned":  "pinned",
				"pin-failed":  "failed",
			}
			var pins []*pinningservice.PinStatus
			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pins = append(pins, pin)
				// this must be "pinned" for the 'pin remote add' command to return
				// after 'pin remote add', we change the status to its real status
				pin.Status = "pinned"
			}

			for pinName := range desiredStatuses {
				node.IPFS("pin", "remote", "add",
					"--service=svc",
					"--name="+pinName,
					hash,
				)
			}
			for _, pin := range pins {
				pin.M.Lock()
				pin.Status = desiredStatuses[pin.Pin.Name]
				pin.M.Unlock()
			}

			res := node.IPFS("pin", "remote", "ls",
				"--service=svc",
				"--status=queued,pinning,pinned,failed",
				"--enc=json",
			)
			actualStatuses := map[string]string{}
			for _, line := range res.Stdout.Lines() {
				name := gjson.Get(line, "Name").Str
				status := gjson.Get(line, "Status").Str
				// drop statuses of other pins we didn't add
				if _, ok := desiredStatuses[name]; ok {
					actualStatuses[name] = status
				}
			}
			assert.Equal(t, desiredStatuses, actualStatuses)
		})

		t.Run("'ipfs pin remote ls' by CID", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			transitionedCh := make(chan struct{}, 1)
			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pin.Status = "pinned"
				transitionedCh <- struct{}{}
			}
			hash := node.IPFSAddStr(string(testutils.RandomBytes(1000)))
			node.IPFS("pin", "remote", "add", "--background=false", "--service=svc", hash)
			<-transitionedCh
			res := node.IPFS("pin", "remote", "ls", "--service=svc", "--cid="+hash, "--enc=json").Stdout.String()
			assert.Contains(t, res, hash)
		})

		t.Run("'ipfs pin remote rm --name' without --force when multiple pins match", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pin.Status = "pinned"
			}
			hash := node.IPFSAddStr(string(testutils.RandomBytes(1000)))
			node.IPFS("pin", "remote", "add", "--service=svc", "--name=force-test-name", hash)
			node.IPFS("pin", "remote", "add", "--service=svc", "--name=force-test-name", hash)

			t.Run("fails", func(t *testing.T) {
				res := node.RunIPFS("pin", "remote", "rm", "--service=svc", "--name=force-test-name")
				assert.Equal(t, 1, res.ExitCode())
				assert.Contains(t, res.Stderr.String(), "Error: multiple remote pins are matching this query, add --force to confirm the bulk removal")
			})

			t.Run("matching pins are not removed", func(t *testing.T) {
				lines := node.IPFS("pin", "remote", "ls", "--service=svc", "--name=force-test-name").Stdout.Lines()
				assert.Contains(t, lines[0], "force-test-name")
				assert.Contains(t, lines[1], "force-test-name")
			})
		})

		t.Run("'ipfs pin remote rm --name --force' remove multiple pins", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pin.Status = "pinned"
			}
			hash := node.IPFSAddStr(string(testutils.RandomBytes(1000)))
			node.IPFS("pin", "remote", "add", "--service=svc", "--name=force-test-name", hash)
			node.IPFS("pin", "remote", "add", "--service=svc", "--name=force-test-name", hash)

			node.IPFS("pin", "remote", "rm", "--service=svc", "--name=force-test-name", "--force")
			out := node.IPFS("pin", "remote", "ls", "--service=svc", "--name=force-test-name").Stdout.Trimmed()
			assert.Empty(t, out)
		})

		t.Run("'ipfs pin remote rm --force' removes all pins", func(t *testing.T) {
			t.Parallel()
			node := harness.NewT(t).NewNode().Init().StartDaemon()
			svc, svcURL := runPinningService(t, authToken)
			node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

			svc.PinAdded = func(req *pinningservice.AddPinRequest, pin *pinningservice.PinStatus) {
				pin.M.Lock()
				defer pin.M.Unlock()
				pin.Status = "pinned"
			}
			for i := 0; i < 4; i++ {
				hash := node.IPFSAddStr(string(testutils.RandomBytes(1000)))
				name := fmt.Sprintf("--name=%d", i)
				node.IPFS("pin", "remote", "add", "--service=svc", "--name="+name, hash)
			}

			lines := node.IPFS("pin", "remote", "ls", "--service=svc").Stdout.Lines()
			assert.Len(t, lines, 4)

			node.IPFS("pin", "remote", "rm", "--service=svc", "--force")

			lines = node.IPFS("pin", "remote", "ls", "--service=svc").Stdout.Lines()
			assert.Len(t, lines, 0)
		})
	})

	t.Run("'ipfs pin remote add' shows a warning message when offline", func(t *testing.T) {
		t.Parallel()
		node := harness.NewT(t).NewNode().Init()
		_, svcURL := runPinningService(t, authToken)
		node.IPFS("pin", "remote", "service", "add", "svc", svcURL, authToken)

		hash := node.IPFSAddStr(string(testutils.RandomBytes(1000)))
		res := node.IPFS("pin", "remote", "add", "--service=svc", "--background", hash)
		warningMsg := "WARNING: the local node is offline and remote pinning may fail if there is no other provider for this CID"
		assert.Contains(t, res.Stdout.String(), warningMsg)
	})
}
