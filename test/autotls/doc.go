// Package autotls contains end-to-end tests for the AutoTLS / p2p-forge
// / ACME issuance chain.
//
// # Why a separate Go module
//
// These tests need to run an in-process Pebble (ACME test server) and the
// full p2p-forge service (CoreDNS-based DNS server + HTTP registration
// endpoint). Pulling those into kubo's main go.mod brings ~350 transitive
// dependencies, including AWS, Azure, GCP, DataDog, and Kubernetes SDKs
// (because CoreDNS supports DNS plugins for all of them). None of that
// belongs in the production `ipfs` binary or in the main module's
// vulnerability-scanning surface.
//
// Isolating the heavy test deps behind their own go.mod keeps:
//
//   - kubo's main module clean: `go build ./cmd/ipfs/` and `go mod tidy` at
//     the repo root are unaffected.
//   - Vulnerability scanning focused: Dependabot does not gain ~350 noisy
//     advisories about cloud SDKs that production code does not use.
//   - `go test ./...` at the repo root fast: this module is not picked up
//     by default. Run it via `make test_autotls` from the repo root,
//     or directly with `cd test/autotls && go test ./...`.
//
// The pattern matches the existing test sub-module in this repo:
// `test/dependencies/go.mod`.
//
// # CI
//
// This module is exercised by a dedicated GitHub Actions job
// (`autotls-tests` in `.github/workflows/gotest.yml`), parallel to the
// main `test_cli` job. The `test_cli` target intentionally does not invoke
// this module, so the CLI suite's compile time stays unaffected by Pebble
// and CoreDNS.
//
// # TODO: consider promoting test/cli to its own module
//
// A cleaner long-term separation would promote the entire `test/cli` tree
// to its own go.mod (with a `replace github.com/ipfs/kubo => ../..`
// directive). That move would isolate every test-only dependency from the
// production module, not just this canary's heavy deps. The cost is
// touching every existing CLI test file and every Makefile / CI target
// that references `test_cli`. Worth doing as its own focused PR.
package autotls
