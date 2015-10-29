package namesys

import (
	"fmt"
	"testing"
	"time"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
	infd "github.com/ipfs/go-ipfs/util/infduration"
)

type mockResolver struct {
	entries map[string]mockEntry
}

type mockEntry struct {
	path string
	ttl  infd.Duration
}

func testResolution(t *testing.T, resolver Resolver, name string, depth int, expected string, expTTL infd.Duration, expError error) {
	p, ttl, err := resolver.ResolveNWithTTL(context.Background(), name, depth)
	if err != expError {
		t.Fatal(fmt.Errorf(
			"Expected %s with a depth of %d to have a '%s' error, but got '%s'",
			name, depth, expError, err))
	}
	if p.String() != expected {
		t.Fatal(fmt.Errorf(
			"%s with depth %d resolved to %s != %s",
			name, depth, p.String(), expected))
	}
	if !infd.Equal(ttl, expTTL) {
		t.Fatal(fmt.Errorf(
			"%s with depth %d had TTL %v != %v",
			name, depth, ttl, expTTL))
	}
}

func (r *mockResolver) resolveOnce(ctx context.Context, name string) (path.Path, infd.Duration, error) {
	entry := r.entries[name]
	p, err := path.ParsePath(entry.path)
	return p, entry.ttl, err
}

func mockResolverOne() *mockResolver {
	return &mockResolver{
		entries: map[string]mockEntry{
			"QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy": mockEntry{"/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.InfiniteDuration()},
			"QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n": mockEntry{"/ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy", infd.FiniteDuration(10 * time.Minute)},
			"QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD": mockEntry{"/ipns/ipfs.io", infd.FiniteDuration(5 * time.Minute)},
		},
	}
}

func mockResolverTwo() *mockResolver {
	return &mockResolver{
		entries: map[string]mockEntry{
			"ipfs.io": mockEntry{"/ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", infd.FiniteDuration(1 * time.Minute)},
		},
	}
}

func TestNamesysResolution(t *testing.T) {
	r := &mpns{
		resolvers: map[string]resolver{
			"one": mockResolverOne(),
			"two": mockResolverTwo(),
		},
	}

	testResolution(t, r, "Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", DefaultDepthLimit, "/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.InfiniteDuration(), nil)
	testResolution(t, r, "/ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy", DefaultDepthLimit, "/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.InfiniteDuration(), nil)
	testResolution(t, r, "/ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", DefaultDepthLimit, "/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.FiniteDuration(10*time.Minute), nil)
	testResolution(t, r, "/ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", 1, "/ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy", infd.FiniteDuration(10*time.Minute), ErrResolveRecursion)
	testResolution(t, r, "/ipns/ipfs.io", DefaultDepthLimit, "/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.FiniteDuration(1*time.Minute), nil)
	testResolution(t, r, "/ipns/ipfs.io", 1, "/ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", infd.FiniteDuration(1*time.Minute), ErrResolveRecursion)
	testResolution(t, r, "/ipns/ipfs.io", 2, "/ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy", infd.FiniteDuration(1*time.Minute), ErrResolveRecursion)
	testResolution(t, r, "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", DefaultDepthLimit, "/ipfs/Qmcqtw8FfrVSBaRmbWwHxt3AuySBhJLcvmFYi3Lbc4xnwj", infd.FiniteDuration(1*time.Minute), nil)
	testResolution(t, r, "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", 1, "/ipns/ipfs.io", infd.FiniteDuration(5*time.Minute), ErrResolveRecursion)
	testResolution(t, r, "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", 2, "/ipns/QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n", infd.FiniteDuration(1*time.Minute), ErrResolveRecursion)
	testResolution(t, r, "/ipns/QmY3hE8xgFCjGcz6PHgnvJz5HZi1BaKRfPkn1ghZUcYMjD", 3, "/ipns/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy", infd.FiniteDuration(1*time.Minute), ErrResolveRecursion)
}
