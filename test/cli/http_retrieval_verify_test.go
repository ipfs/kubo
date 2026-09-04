package cli

import (
	crand "crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ipfs/boxo/routing/http/server"
	"github.com/ipfs/boxo/routing/http/types"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/ipfs/kubo/test/cli/testutils/httprouting"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// HTTP retrieval lets Kubo fetch blocks straight from HTTP gateways that
// advertise themselves through /routing/v1 (config: HTTPRetrieval.Enabled).
// Kubo relies on this path more over time, and #11333 adds the serving side: a
// Kubo node can serve blocks over native HTTPS. As the set of HTTP providers
// grows, more of them run on machines and software outside the project's
// control.
//
// An HTTP endpoint can return the wrong bytes for many ordinary reasons:
// a caching proxy in front of it, a misconfigured rewrite, a truncated
// response, a buggy or hostile operator. Kubo treats provider bytes as
// untrusted and verifies they hash to the requested CID before handing them to
// the user. That check is the content-addressing guarantee.
//
// These tests are belt-and-suspenders coverage that the guarantee holds end to
// end (the Kubo daemon, its bitswap HTTP client in boxo, and the /routing/v1
// lookup), so a regression anywhere in that chain (in Kubo or an upstream
// library) trips here instead of reaching a user as silently corrupt data.
// They run on loopback with no internet dependency.
//
// Out of scope: trustless CAR responses (the bitswap HTTP client requests raw
// blocks only) and multi-block DAGs. A single raw block is the smallest case
// that exercises fetch-and-verify, so that is what these use.

// httpRetrievalNoLeakTimeout bounds the "fails closed" assertion below. When the
// only provider serves corrupt bytes, bitswap keeps re-finding and re-trying it,
// so `ipfs cat` never returns on its own. We cap it and assert it failed without
// emitting the bad bytes. It only needs to be long enough for the daemon to
// connect to the provider and fetch (then reject) the block at least once, which
// happens in about a second on loopback; 10s leaves ample margin on slow CI.
const httpRetrievalNoLeakTimeout = 10 * time.Second

// httpnetPingCID is the empty identity block that boxo's bitswap HTTP client
// (bitswap/network/httpnet, var pingCid) GETs to probe whether a provider is
// reachable before fetching real blocks.
const httpnetPingCID = "bafkqaaa"

// TestHTTPRetrievalRejectsBytesThatDoNotMatchCID checks that when the only
// provider for a CID is an HTTP gateway returning bytes that do not hash to that
// CID, Kubo fails the retrieval instead of returning the corrupt bytes.
//
// Content addressing must hold even when the only source available is broken or
// lying: better a failed fetch than silently corrupt data.
func TestHTTPRetrievalRejectsBytesThatDoNotMatchCID(t *testing.T) {
	t.Parallel()
	debug := os.Getenv("DEBUG") == "true"

	node, router := newHTTPRetrievalNode(t, debug)

	// Compute the CID of the real content without storing it locally (-n), so
	// the daemon must fetch it from the (only) HTTP provider. A small CIDv1
	// input becomes a single raw block whose bytes are exactly the content.
	content := uniq("kubo http retrieval rejects bytes that do not match the requested cid")
	testCID := cid.MustParse(node.PipeStrToIPFS(content, "add", "-qn", "--cid-version=1").Stdout.Trimmed())

	// One provider, and it serves bytes that do not match testCID.
	corrupt := []byte("these bytes deliberately do not hash to the requested CID")
	badProvider := newMockGatewayProvider(t, testCID, corrupt, debug)
	badPeerID := registerHTTPProvider(t, router, testCID, badProvider.multiaddr(t))

	node.StartDaemon()
	defer node.StopDaemon()

	// The provider is discoverable, so the daemon has something to try.
	findprovs := node.IPFS("routing", "findprovs", testCID.String())
	require.Equal(t, badPeerID.String(), findprovs.Stdout.Trimmed(),
		"the broken provider should be discoverable via /routing/v1")

	// Attempt retrieval. The only provider keeps failing verification, so the
	// daemon never satisfies the request; --timeout bounds the wait.
	res := node.RunIPFS("cat", "--timeout="+httpRetrievalNoLeakTimeout.String(), testCID.String())

	// It must fail, and crucially it must not emit the corrupt bytes.
	require.NotNil(t, res.ExitErr,
		"ipfs cat must fail when the only provider serves bytes that do not match the CID (stdout=%q stderr=%q)",
		res.Stdout.String(), res.Stderr.String())
	require.Empty(t, res.Stdout.Bytes(),
		"ipfs cat must not output anything that failed CID verification")

	// Prove the daemon actually fetched the block from the provider and rejected
	// it, rather than simply never reaching it.
	require.Positive(t, badProvider.blockGETs.Load(),
		"the daemon should have fetched the block from the provider and rejected it on hash mismatch")
}

// TestHTTPRetrievalFailsOverFromCorruptProvider checks that one bad HTTP
// provider does not poison retrieval: when a CID is served by both a broken
// provider (wrong bytes) and a valid one, Kubo returns the valid bytes.
//
// A single misbehaving provider in a set should not deny the content. Kubo must
// reject its bytes and fetch the real block from a provider that serves it.
func TestHTTPRetrievalFailsOverFromCorruptProvider(t *testing.T) {
	t.Parallel()
	debug := os.Getenv("DEBUG") == "true"

	node, router := newHTTPRetrievalNode(t, debug)

	content := uniq("kubo http retrieval fails over from a corrupt provider to a valid one")
	testCID := cid.MustParse(node.PipeStrToIPFS(content, "add", "-qn", "--cid-version=1").Stdout.Trimmed())

	// Register the broken provider first, exercising the worst case where it may
	// be contacted before the valid one.
	corrupt := []byte("these bytes deliberately do not hash to the requested CID")
	badProvider := newMockGatewayProvider(t, testCID, corrupt, debug)
	registerHTTPProvider(t, router, testCID, badProvider.multiaddr(t))

	// And a valid provider that serves the real content.
	goodProvider := newMockGatewayProvider(t, testCID, []byte(content), debug)
	registerHTTPProvider(t, router, testCID, goodProvider.multiaddr(t))

	node.StartDaemon()
	defer node.StopDaemon()

	// Retrieval should succeed with the correct bytes. The generous --timeout is
	// a safety bound so a regression surfaces as a failure rather than a hang.
	res := node.RunIPFS("cat", "--timeout=60s", testCID.String())
	require.Nil(t, res.ExitErr,
		"ipfs cat should succeed via the valid provider (stderr=%q)", res.Stderr.String())
	require.Equal(t, content, res.Stdout.Trimmed(),
		"ipfs cat must return the bytes that match the CID, from the valid provider")
}

// newHTTPRetrievalNode returns an initialized (not yet started) Kubo node wired
// to fetch blocks over HTTP, and the in-process /routing/v1 mock that decides
// which providers it discovers. The caller computes CIDs, registers providers,
// then starts the daemon.
func newHTTPRetrievalNode(t *testing.T, debug bool) (*harness.Node, *httprouting.MockHTTPContentRouter) {
	t.Helper()

	router := &httprouting.MockHTTPContentRouter{Debug: debug}
	routerSrv := httptest.NewServer(server.Handler(router))
	t.Cleanup(routerSrv.Close)

	node := harness.NewT(t).NewNode().Init()
	node.UpdateConfig(func(cfg *config.Config) {
		// Fetch blocks directly from HTTP providers found via /routing/v1.
		cfg.HTTPRetrieval.Enabled = config.True
		// Our mock providers serve over self-signed TLS.
		cfg.HTTPRetrieval.TLSInsecureSkipVerify = config.True
		// Resolve providers only through the in-process mock router, so the test
		// needs no internet and stays deterministic.
		cfg.Routing.DelegatedRouters = []string{routerSrv.URL}
	})
	return node, router
}

// registerHTTPProvider advertises srvMultiaddr as a /routing/v1 provider for c
// and returns the peer ID it was given. The record matches what a real HTTP
// gateway publishes (Schema "peer", an /https multiaddr, and the
// transport-ipfs-gateway-http protocol).
func registerHTTPProvider(t *testing.T, router *httprouting.MockHTTPContentRouter, c cid.Cid, srvMultiaddr multiaddr.Multiaddr) peer.ID {
	t.Helper()
	pid := randomPeerID(t)
	router.AddProvider(c, &types.PeerRecord{
		Schema:    types.SchemaPeer,
		ID:        &pid,
		Addrs:     []types.Multiaddr{{Multiaddr: srvMultiaddr}},
		Protocols: []string{"transport-ipfs-gateway-http"},
	})
	return pid
}

// randomPeerID returns a valid, unique peer ID. Its value is irrelevant to HTTP
// retrieval (the multiaddr drives the connection); it only needs to be distinct
// so two providers are two separate records.
func randomPeerID(t *testing.T) peer.ID {
	t.Helper()
	_, pub, err := crypto.GenerateEd25519Key(crand.Reader)
	require.NoError(t, err)
	pid, err := peer.IDFromPublicKey(pub)
	require.NoError(t, err)
	return pid
}

// mockGatewayProvider is a minimal trustless HTTP gateway standing in for an
// HTTP provider discovered via /routing/v1. It answers block requests for one
// target CID (GET/HEAD /ipfs/<cid>?format=raw, application/vnd.ipld.raw) with a
// fixed payload, and answers the httpnet connect probe (the empty bafkqaaa
// identity block) so the daemon treats it as reachable.
//
// The payload is set by the caller, so a provider can be told to return bytes
// that do not hash to the target CID, standing in for an HTTP endpoint that
// serves wrong or corrupted data.
type mockGatewayProvider struct {
	server    *httptest.Server
	blockGETs atomic.Int64 // block GETs served, proof the daemon fetched our bytes
}

func newMockGatewayProvider(t *testing.T, target cid.Cid, payload []byte, debug bool) *mockGatewayProvider {
	t.Helper()
	p := &mockGatewayProvider{}
	blockPath := "/ipfs/" + target.String()
	probePath := "/ipfs/" + httpnetPingCID

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if debug {
			fmt.Printf("mockGatewayProvider %s %s\n", r.Method, r.URL.Path)
		}
		switch {
		case strings.HasPrefix(r.URL.Path, blockPath):
			w.Header().Set("Content-Type", "application/vnd.ipld.raw")
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodGet {
				p.blockGETs.Add(1)
				_, _ = w.Write(payload)
			}
		case strings.HasPrefix(r.URL.Path, probePath):
			// Reachability probe: any 200 marks the provider connected.
			w.Header().Set("Content-Type", "application/vnd.ipld.raw")
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	})

	// Self-signed TLS + HTTP/2, matching how real providers serve over https.
	// The daemon is configured with HTTPRetrieval.TLSInsecureSkipVerify=true.
	srv := httptest.NewUnstartedServer(handler)
	srv.EnableHTTP2 = true
	srv.StartTLS()
	t.Cleanup(srv.Close)

	p.server = srv
	return p
}

// multiaddr returns the provider's address in the shape a real HTTP gateway
// publishes: /ip4/<host>/tcp/<port>/https. Real records often use
// /dns/<host>/tcp/443/https; a loopback IP keeps the test free of any DNS or
// internet dependency while taking the same /https code path.
func (p *mockGatewayProvider) multiaddr(t *testing.T) multiaddr.Multiaddr {
	t.Helper()
	host, port, err := splitHostPort(p.server.URL)
	require.NoError(t, err)
	ma, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s/https", host, port))
	require.NoError(t, err)
	return ma
}
