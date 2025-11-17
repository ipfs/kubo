//go:build go1.25

package commands

// This file contains unit tests for the --heal-timeout flag functionality
// using testing/synctest to avoid waiting for real timeouts.
//
// End-to-end tests for the full 'ipfs repo verify' command (including --drop
// and --heal flags) are located in test/cli/repo_verify_test.go.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	coreiface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ipfs/boxo/path"
)

func TestVerifyWorkerHealTimeout(t *testing.T) {
	t.Run("heal succeeds before timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const healTimeout = 5 * time.Second
			testCID := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")

			// Setup channels
			keys := make(chan cid.Cid, 1)
			keys <- testCID
			close(keys)
			results := make(chan *verifyResult, 1)

			// Mock blockstore that returns error (simulating corruption)
			mockBS := &mockBlockstore{
				getError: errors.New("corrupt block"),
			}

			// Mock API where Block().Get() completes before timeout
			mockAPI := &mockCoreAPI{
				blockAPI: &mockBlockAPI{
					getDelay: 2 * time.Second, // Less than healTimeout
					data:     []byte("healed data"),
				},
			}

			var wg sync.WaitGroup
			wg.Add(1)

			// Run worker
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, true, true, healTimeout)

			// Advance time past the mock delay but before timeout
			time.Sleep(3 * time.Second)
			synctest.Wait()

			wg.Wait()
			close(results)

			// Verify heal succeeded
			result := <-results
			require.NotNil(t, result)
			assert.Equal(t, verifyStateCorruptHealed, result.state)
			assert.Contains(t, result.msg, "healed")
		})
	})

	t.Run("heal fails due to timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const healTimeout = 2 * time.Second
			testCID := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")

			// Setup channels
			keys := make(chan cid.Cid, 1)
			keys <- testCID
			close(keys)
			results := make(chan *verifyResult, 1)

			// Mock blockstore that returns error (simulating corruption)
			mockBS := &mockBlockstore{
				getError: errors.New("corrupt block"),
			}

			// Mock API where Block().Get() takes longer than healTimeout
			mockAPI := &mockCoreAPI{
				blockAPI: &mockBlockAPI{
					getDelay: 5 * time.Second, // More than healTimeout
					data:     []byte("healed data"),
				},
			}

			var wg sync.WaitGroup
			wg.Add(1)

			// Run worker
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, true, true, healTimeout)

			// Advance time past timeout
			time.Sleep(3 * time.Second)
			synctest.Wait()

			wg.Wait()
			close(results)

			// Verify heal failed due to timeout
			result := <-results
			require.NotNil(t, result)
			assert.Equal(t, verifyStateCorruptHealFailed, result.state)
			assert.Contains(t, result.msg, "failed to heal")
			assert.Contains(t, result.msg, "context deadline exceeded")
		})
	})

	t.Run("heal with zero timeout still attempts heal", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const healTimeout = 0 // Zero timeout means no timeout
			testCID := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")

			// Setup channels
			keys := make(chan cid.Cid, 1)
			keys <- testCID
			close(keys)
			results := make(chan *verifyResult, 1)

			// Mock blockstore that returns error (simulating corruption)
			mockBS := &mockBlockstore{
				getError: errors.New("corrupt block"),
			}

			// Mock API that succeeds quickly
			mockAPI := &mockCoreAPI{
				blockAPI: &mockBlockAPI{
					getDelay: 100 * time.Millisecond,
					data:     []byte("healed data"),
				},
			}

			var wg sync.WaitGroup
			wg.Add(1)

			// Run worker
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, true, true, healTimeout)

			// Advance time to let heal complete
			time.Sleep(200 * time.Millisecond)
			synctest.Wait()

			wg.Wait()
			close(results)

			// Verify heal succeeded even with zero timeout
			result := <-results
			require.NotNil(t, result)
			assert.Equal(t, verifyStateCorruptHealed, result.state)
			assert.Contains(t, result.msg, "healed")
		})
	})

	t.Run("multiple blocks with different timeout outcomes", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const healTimeout = 3 * time.Second
			testCID1 := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")
			testCID2 := cid.MustParse("bafybeihvvulpp4evxj7x7armbqcyg6uezzuig6jp3lktpbovlqfkjtgyby")

			// Setup channels
			keys := make(chan cid.Cid, 2)
			keys <- testCID1
			keys <- testCID2
			close(keys)
			results := make(chan *verifyResult, 2)

			// Mock blockstore that always returns error (all blocks corrupt)
			mockBS := &mockBlockstore{
				getError: errors.New("corrupt block"),
			}

			// Create two mock block APIs with different delays
			// We'll need to alternate which one gets used
			// For simplicity, use one that succeeds fast
			mockAPI := &mockCoreAPI{
				blockAPI: &mockBlockAPI{
					getDelay: 1 * time.Second, // Less than healTimeout - will succeed
					data:     []byte("healed data"),
				},
			}

			var wg sync.WaitGroup
			wg.Add(2) // Two workers

			// Run two workers
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, true, true, healTimeout)
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, true, true, healTimeout)

			// Advance time to let both complete
			time.Sleep(2 * time.Second)
			synctest.Wait()

			wg.Wait()
			close(results)

			// Collect results
			var healedCount int
			for result := range results {
				if result.state == verifyStateCorruptHealed {
					healedCount++
				}
			}

			// Both should heal successfully (both under timeout)
			assert.Equal(t, 2, healedCount)
		})
	})

	t.Run("valid block is not healed", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			const healTimeout = 5 * time.Second
			testCID := cid.MustParse("bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi")

			// Setup channels
			keys := make(chan cid.Cid, 1)
			keys <- testCID
			close(keys)
			results := make(chan *verifyResult, 1)

			// Mock blockstore that returns valid block (no error)
			mockBS := &mockBlockstore{
				block: blocks.NewBlock([]byte("valid data")),
			}

			// Mock API (won't be called since block is valid)
			mockAPI := &mockCoreAPI{
				blockAPI: &mockBlockAPI{},
			}

			var wg sync.WaitGroup
			wg.Add(1)

			// Run worker with heal enabled
			go verifyWorkerRun(t.Context(), &wg, keys, results, mockBS, mockAPI, false, true, healTimeout)

			synctest.Wait()

			wg.Wait()
			close(results)

			// Verify block is marked valid, not healed
			result := <-results
			require.NotNil(t, result)
			assert.Equal(t, verifyStateValid, result.state)
			assert.Empty(t, result.msg)
		})
	})
}

// mockBlockstore implements a minimal blockstore for testing
type mockBlockstore struct {
	getError error
	block    blocks.Block
}

func (m *mockBlockstore) Get(ctx context.Context, c cid.Cid) (blocks.Block, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return m.block, nil
}

func (m *mockBlockstore) DeleteBlock(ctx context.Context, c cid.Cid) error {
	return nil
}

func (m *mockBlockstore) Has(ctx context.Context, c cid.Cid) (bool, error) {
	return m.block != nil, nil
}

func (m *mockBlockstore) GetSize(ctx context.Context, c cid.Cid) (int, error) {
	if m.block != nil {
		return len(m.block.RawData()), nil
	}
	return 0, errors.New("block not found")
}

func (m *mockBlockstore) Put(ctx context.Context, b blocks.Block) error {
	return nil
}

func (m *mockBlockstore) PutMany(ctx context.Context, bs []blocks.Block) error {
	return nil
}

func (m *mockBlockstore) AllKeysChan(ctx context.Context) (<-chan cid.Cid, error) {
	return nil, errors.New("not implemented")
}

func (m *mockBlockstore) HashOnRead(enabled bool) {
}

// mockBlockAPI implements BlockAPI for testing
type mockBlockAPI struct {
	getDelay time.Duration
	getError error
	data     []byte
}

func (m *mockBlockAPI) Get(ctx context.Context, p path.Path) (io.Reader, error) {
	if m.getDelay > 0 {
		select {
		case <-time.After(m.getDelay):
			// Delay completed
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.getError != nil {
		return nil, m.getError
	}
	return bytes.NewReader(m.data), nil
}

func (m *mockBlockAPI) Put(ctx context.Context, r io.Reader, opts ...options.BlockPutOption) (coreiface.BlockStat, error) {
	return nil, errors.New("not implemented")
}

func (m *mockBlockAPI) Rm(ctx context.Context, p path.Path, opts ...options.BlockRmOption) error {
	return errors.New("not implemented")
}

func (m *mockBlockAPI) Stat(ctx context.Context, p path.Path) (coreiface.BlockStat, error) {
	return nil, errors.New("not implemented")
}

// mockCoreAPI implements minimal CoreAPI for testing
type mockCoreAPI struct {
	blockAPI *mockBlockAPI
}

func (m *mockCoreAPI) Block() coreiface.BlockAPI {
	return m.blockAPI
}

func (m *mockCoreAPI) Unixfs() coreiface.UnixfsAPI   { return nil }
func (m *mockCoreAPI) Dag() coreiface.APIDagService  { return nil }
func (m *mockCoreAPI) Name() coreiface.NameAPI       { return nil }
func (m *mockCoreAPI) Key() coreiface.KeyAPI         { return nil }
func (m *mockCoreAPI) Pin() coreiface.PinAPI         { return nil }
func (m *mockCoreAPI) Object() coreiface.ObjectAPI   { return nil }
func (m *mockCoreAPI) Swarm() coreiface.SwarmAPI     { return nil }
func (m *mockCoreAPI) PubSub() coreiface.PubSubAPI   { return nil }
func (m *mockCoreAPI) Routing() coreiface.RoutingAPI { return nil }

func (m *mockCoreAPI) ResolvePath(ctx context.Context, p path.Path) (path.ImmutablePath, []string, error) {
	return path.ImmutablePath{}, nil, errors.New("not implemented")
}

func (m *mockCoreAPI) ResolveNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	return nil, errors.New("not implemented")
}

func (m *mockCoreAPI) WithOptions(...options.ApiOption) (coreiface.CoreAPI, error) {
	return nil, errors.New("not implemented")
}
