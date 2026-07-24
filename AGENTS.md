# AI Agent Instructions for Kubo

This file provides instructions for AI coding agents working on the [Kubo](https://github.com/ipfs/kubo) codebase (the Go implementation of IPFS). Follow the [Developer Guide](docs/developer-guide.md) for full details.

## Quick Reference

| Task              | Command                                                  |
|-------------------|----------------------------------------------------------|
| Tidy deps         | `make mod_tidy` (all modules; required if deps changed)  |
| Build             | `make build`                                             |
| Unit tests        | `go test ./... -run TestName -v`                         |
| Integration tests | `make build && go test ./test/cli/... -run TestName -v`  |
| Lint              | `make -O test_go_lint`                                   |
| Format            | `go fmt ./...`                                           |

## Project Overview

Kubo is the reference implementation of IPFS in Go. Most IPFS protocol logic lives in [boxo](https://github.com/ipfs/boxo) (the IPFS SDK); kubo wires it together and exposes it via CLI and HTTP RPC API. Before adding protocol logic here, check whether it belongs in boxo (see [Where a change belongs](#where-a-change-belongs-boxo-or-kubo)).

Key directories:

| Directory          | Purpose                                                  |
|--------------------|----------------------------------------------------------|
| `cmd/ipfs/`        | CLI entry point and binary                               |
| `core/`            | core IPFS node implementation                            |
| `core/commands/`   | CLI command definitions                                  |
| `core/coreapi/`    | Go API implementation                                    |
| `client/rpc/`      | HTTP RPC client                                          |
| `plugin/`          | plugin system                                            |
| `repo/`            | repository management                                    |
| `test/cli/`        | Go-based CLI integration tests (preferred for new tests) |
| `test/sharness/`   | legacy shell-based integration tests                     |
| `docs/`            | documentation                                            |

Other key external dependencies: [go-libp2p](https://github.com/libp2p/go-libp2p) (networking), [go-libp2p-kad-dht](https://github.com/libp2p/go-libp2p-kad-dht) (DHT).

## Where a change belongs: boxo or kubo

[boxo](https://github.com/ipfs/boxo) is the Go SDK for IPFS: a set of libraries for building IPFS applications and implementations. Kubo is one consumer of boxo, not the only one, so reusable building blocks live in boxo where other Go software can use them without pulling in kubo.

- **Goes in boxo:** protocol logic and reusable primitives another Go program could use on its own, for example Bitswap, UnixFS, the HTTP gateway, IPLD and path helpers, routing and provider systems, MFS, and the blockstore and blockservice layers. If the code does not depend on kubo's config, CLI, or daemon and would help someone building a different tool, it belongs in boxo.
- **Goes in kubo:** the daemon and product, for example the config schema, CLI commands (`core/commands/`), the `/api/v0/` RPC surface, node construction and lifecycle, the on-disk repo, plugins, and migrations. Kubo-specific product decisions stay here.
- **The usual shape of a feature:** build the reusable capability in boxo, then wire it into kubo (config option, CLI or RPC surface, `docs/config.md` entry). If you are adding generic protocol logic under `core/`, stop and ask whether it belongs in boxo instead.

Not everything IPFS-related belongs in boxo; its README lists the inclusion criteria. When unsure, open an issue before building, but do not trap generic, reusable logic inside kubo.

## Stability: What You Must Not Break

Backward compatibility is the top priority, above new features and above internal elegance. [CONTRIBUTING.md](CONTRIBUTING.md) explains why the project holds this line and who it is for. The hard rules an agent must not cross:

- **Never break the `/api/v0/` RPC API.** This is Kubo's own RPC interface, not a shared IPFS protocol, and other implementations are not expected to provide it. That is exactly why it must not change: more than a decade of software is built against Kubo's specific API, including [ipfs-cluster](https://github.com/ipfs-cluster/ipfs-cluster), [IPFS Desktop](https://github.com/ipfs/ipfs-desktop), [IPFS Companion](https://github.com/ipfs/ipfs-companion), orchestration scripts, and third-party libraries in many languages. Adding a new endpoint or a new optional argument is fine; removing an endpoint, renaming a field, changing a default, or altering a response shape is not. The Go `CoreAPI` interfaces in `core/coreiface/` (implemented by `core/coreapi/` and the RPC client in `client/rpc/`) follow the same rule.
- **Never break the HTTP Gateway.** Unlike the RPC API, the gateway served on `Addresses.Gateway` is not Kubo-specific: it is a generic, vendor-neutral HTTP interface defined by the [HTTP Gateway specs](https://specs.ipfs.tech/http-gateways/) and implemented by many gateways and tools. Browsers, apps, and tooling depend on its response headers, status codes, and URL conventions (path, subdomain, DNSLink, and trustless gateways). Kubo must stay conformant; changing gateway behavior in a way the specs do not allow is a breaking change, and like any protocol change it goes through an IPIP first. Conformance is checked in CI on pull requests by the [`ipfs/gateway-conformance`](https://github.com/ipfs/gateway-conformance) suite (`.github/workflows/gateway-conformance.yml`), with local coverage in `test/cli/gateway_test.go`; a failing conformance run means you broke the contract.
- **Never change the default CID recipe.** The default `ipfs add` recipe (CID version, chunker, hash, DAG layout) must keep producing the same CID for the same bytes; changing a default silently forks the address space. The recipes are named and documented in [IPIP-0499: UnixFS CID Profiles](https://specs.ipfs.tech/ipips/ipip-0499/); Kubo's current default matches the legacy `unixfs-v0-2015` profile, pinned by `test/cli/cid_profiles_test.go` (`TestDefaultMatchesExpectedProfile`). New recipes ship as opt-in profiles.
- **Protocol changes need an IPIP first.** A change to how Kubo talks to other implementations on the wire (a new protocol, a change to an existing one, a new field peers must understand) needs an IPIP (InterPlanetary Improvement Proposal) in [ipfs/specs](https://github.com/ipfs/specs/) before it ships. [specs.ipfs.tech](https://specs.ipfs.tech/) is the source of truth for IPFS protocols; Kubo implements them, it does not define them unilaterally.
- **Every hardcoded endpoint or shared-infrastructure dependency must be configurable and possible to turn off.** If you add code that talks to a fixed URL, a default bootstrap peer, a delegated router, a certificate authority, or any semi-centralized or federated service, expose it in `docs/config.md` with an override and an off switch. `AutoConf` (see `docs/config.md`) is the model: default network infrastructure is fetched from a configurable endpoint, every value can be overridden locally, and the whole system can be disabled. A node operator must never be locked into an endpoint the maintainers picked.

When a breaking change is unavoidable, it does not go in quietly: it needs maintainer sign-off, a migration path, a changelog entry spelling out the impact, and usually a deprecation period first. When you are unsure whether a change breaks compatibility or needs an IPIP, open an issue at <https://github.com/ipfs/kubo/issues> before writing code; it probably does.

## Go Style

Follow these Go style references:

- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Google Go Style Decisions](https://google.github.io/styleguide/go/decisions)

Specific conventions for this project:

- check the Go version in `go.mod` and use idiomatic features available at that version
- readability over micro-optimization: clear code is more important than saving microseconds
- prefer standard library functions and utilities over writing your own
- use early returns and indent the error flow, not the happy path
- use `slices.Contains`, `slices.DeleteFunc`, and the `maps` package instead of manual loops
- preallocate slices and maps when the size is known: `make([]T, 0, n)`
- use `map[K]struct{}` for sets, not `map[K]bool`
- receiver names: single-letter abbreviations matching the type (e.g., `s *Server`, `c *Client`)
- run `go fmt` after modifying Go source files, never indent manually

### Error Handling

- wrap errors with `fmt.Errorf("context: %w", err)`, never discard errors silently
- use `errors.Is` / `errors.As` for error checking, not string comparison
- never use `panic` in library code; only in `main` or test helpers
- return `nil` explicitly for the error value on success paths

### Canonical Examples

When adding or modifying code, follow the patterns established in these files:

- CLI command structure: `core/commands/dag/dag.go`
- CLI integration test: `test/cli/dag_test.go`
- Test harness usage: `test/cli/harness/` package

## Building

Always run commands from the repository root.

```bash
make mod_tidy        # update go.mod/go.sum (use this instead of go mod tidy)
make build           # build the ipfs binary to cmd/ipfs/ipfs
make install         # install to $GOPATH/bin
make -O test_go_lint # run linter (use this instead of golangci-lint directly)
```

**Always build with `make build`, never `go build`.** The Makefile injects required `-ldflags` for `CurrentCommit`, `taggedRelease`, and `buildOrigin`.

If you change dependencies in any `go.mod`, you must run `make mod_tidy`, and you must run it before committing, pushing, or opening a PR. The repo has three `go.mod` files (root, `docs/examples/kubo-as-a-library`, and `test/dependencies`) that have to stay on the same dependency versions. `make mod_tidy` runs `go mod tidy` in every one of them; a bare `go mod tidy` only touches the module you run it in, which lets the pins drift out of sync between modules (for example the root pointing at one boxo commit while `test/dependencies` points at another). Run it before building or testing too, since it also updates `go.sum`.

If you modify any `.go` files outside of `test/`, you must run `make build` before running integration tests.

## Testing

The full test suite is composed of several targets:

| Make target          | What it runs                                                          |
|----------------------|-----------------------------------------------------------------------|
| `make test`          | all tests (`test_go_fmt` + `test_unit` + `test_cli` + `test_sharness`) |
| `make test_short`    | fast subset (`test_go_fmt` + `test_unit`)                             |
| `make test_unit`     | unit tests with coverage (excludes `test/cli`)                        |
| `make test_cli`      | CLI integration tests (requires `make build` first)                   |
| `make test_fuse`     | FUSE filesystem tests (requires `/dev/fuse` and `fusermount` in PATH) |
| `make test_sharness` | legacy shell-based integration tests                                  |
| `make test_go_fmt`   | checks Go source formatting                                          |
| `make -O test_go_lint` | runs `golangci-lint`                                                |

During development, prefer running a specific test rather than the full suite:

```bash
# run a single unit test
go test ./core/... -run TestSpecificUnit -v

# run a single CLI integration test (requires make build first)
go test ./test/cli/... -run TestSpecificCLI -v
```

### Environment Setup for Integration Tests

Before running `test_cli` or `test_sharness`, set these environment variables from the repo root:

```bash
export PATH="$PWD/cmd/ipfs:$PATH"
export IPFS_PATH="$(mktemp -d)"
```

- `PATH`: integration tests use the `ipfs` binary from `PATH`, not Go source directly
- `IPFS_PATH`: isolates test data from `~/.ipfs` or other running nodes

If you see "version (N) is lower than repos (M)", the `ipfs` binary in `PATH` is outdated. Rebuild with `make build` and verify `PATH`.

### Running FUSE Tests

FUSE tests require `/dev/fuse` and `fusermount` in `PATH`. On systems with only fuse3, create a symlink in a temp directory (never use `sudo` to install system-wide):

```bash
FUSE_BIN="$(mktemp -d)" && ln -s /usr/bin/fusermount3 "$FUSE_BIN/fusermount" && PATH="$FUSE_BIN:$PATH" make test_fuse
```

Set `TEST_FUSE=1` to make mount failures fatal (CI does this). Without it, tests auto-detect and skip when FUSE is unavailable.

### Running Sharness Tests

Sharness tests are legacy shell-based tests. Run individual tests with a timeout:

```bash
cd test/sharness && timeout 60s ./t0080-repo.sh
```

To investigate a failing test, pass `-v` for verbose output. In this mode, daemons spawned by the test are not shut down automatically and must be killed manually afterwards.

### Cleaning Up Stale Daemons

Before running `test/cli` or `test/sharness`, stop any stale `ipfs daemon` processes owned by the current user. Leftover daemons hold locks and bind ports, causing test failures:

```bash
pkill -f "ipfs daemon"
```

### Writing Tests

- all new integration tests go in `test/cli/`, not `test/sharness/`
- if a `test/sharness` test needs significant changes, remove it and add a replacement in `test/cli/`
- use [testify](https://github.com/stretchr/testify) for assertions (already a dependency)
- use `t.Context()` instead of `context.Background()` in tests
- for Go 1.25+, use `testing/synctest` when testing concurrent code (goroutines, channels, timers)
- reuse existing `.car` fixtures in `test/cli/fixtures/` when possible; only add new fixtures when the test requires data not covered by existing ones
- when writing tests that cover CIDv0 vs CIDv1, always set the CID version explicitly (never rely on defaults); if chunk size matters for the test, also set the chunker explicitly
- always re-run modified tests locally before submitting to confirm they pass
- avoid emojis in test names and test log output

## Before Submitting

Run these steps in order before committing, pushing, or opening a PR:

1. `make mod_tidy` (required whenever any `go.mod` changed, so all three modules stay in sync)
2. `go fmt ./...`
3. `make build` (if non-test `.go` files changed)
4. `make -O test_go_lint`
5. `go test ./...` (or the relevant subset)

## Documentation and Commit Messages

- after editing CLI help text in `core/commands/`, verify width: `go test ./test/cli/... -run TestCommandDocsWidth`
- **CLI `--help` text and RPC command descriptions are user-facing documentation.** The reference pages at [docs.ipfs.tech/reference/kubo/cli](https://docs.ipfs.tech/reference/kubo/cli/) and [docs.ipfs.tech/reference/kubo/rpc](https://docs.ipfs.tech/reference/kubo/rpc/) are generated from the command definitions in `core/commands/` by a CI job in [ipfs/ipfs-docs](https://github.com/ipfs/ipfs-docs) after each release. Whatever you put in a command's `Helptext` is what users read on the website, so keep it accurate and complete. Where a command implements a spec or a non-obvious concept, link to [specs.ipfs.tech](https://specs.ipfs.tech/) or the relevant docs so a reader can learn the "why", not just the syntax.
- **`docs/config.md` is where users learn how Kubo works, not just a list of keys.** It is a common entry point for understanding a feature. When you add or change a config option, document what it does and why someone would touch it, and link out to the spec or educational material behind the concept. A new or changed option without a matching `docs/config.md` entry is an incomplete change.
- changelogs in `docs/changelogs/`: only edit the Table of Contents and the Highlights section; the Changelog and Contributors sections are auto-generated and must not be modified
- avoid unnecessary line wrapping in `docs/changelogs/*`; let lines be long
- follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
- keep commit titles short and messages terse

## Writing Style

When writing docs, comments, and commit messages:

- avoid emojis in code, comments, and log output
- keep an empty line before lists in markdown
- use backticks around CLI commands, paths, environment variables, and config options

## PR Guidelines

Every PR needs a description and tests. These are not optional; a change with neither is not reviewable and should not be merged.

- explain what changed and why in the PR description, so a reviewer who was not in the discussion can understand it
- include test coverage for new functionality and bug fixes; a bug fix without a test that would have caught the bug is incomplete
- new integration tests go in `test/cli/`, not `test/sharness/` (see [Writing Tests](#writing-tests) for what to do when an existing `test/sharness` test needs changes)
- run `make -O test_go_lint` and fix any lint issues before submitting
- verify that `go test ./...` passes locally
- end the PR description with a `## References` section listing related context, one link per line
- if the PR closes an issue in `ipfs/kubo`, each closing reference should be a bullet starting with `Closes`:

```markdown
## References

- Closes https://github.com/ipfs/kubo/issues/1234
- Closes https://github.com/ipfs/kubo/issues/5678
- https://discuss.ipfs.tech/t/related-topic/999
```

## Scope and Safety

Do not modify or touch:

- files under `test/sharness/lib/` (third-party sharness test framework)
- CI workflows in `.github/` unless explicitly asked
- auto-generated sections in `docs/changelogs/` (Changelog and Contributors are generated; only TOC and Highlights are human-edited)

Releases are maintainer-driven and follow [`docs/RELEASE_CHECKLIST.md`](docs/RELEASE_CHECKLIST.md). Unless you are running a release, do not bump `version.go`, touch release tooling (`bin/mkreleaselog`, the release workflows), or push tags; pushing a tag sets off release publishing (Docker Hub, npm, and dist.ipfs.tech) and cannot be undone.

Do not run without being asked:

- `make test` or `make test_sharness` (full suite is slow; prefer targeted tests)
- `ipfs daemon` without a timeout

## Running the Daemon

Always run the daemon with a timeout or shut it down promptly:

```bash
timeout 60s ipfs daemon   # auto-kill after 60s
ipfs shutdown              # graceful shutdown via API
```

Kill dangling daemons before re-running tests: `pkill -f "ipfs daemon"`

### Use Non-Default Ports for Manual Experiments

A real IPFS node may already be running on the host using the default ports: swarm `4001`, RPC API `5001`, and gateway `8080`. Any manual experiment, PoC, or benchmark daemon you start MUST use non-default ports (and its own `IPFS_PATH`) so it does not collide with or disrupt that node. Binding a default port fails with `address already in use`, and reusing another node's API can interfere with it.

```bash
export IPFS_PATH="$(mktemp -d)"
ipfs init >/dev/null
ipfs config --json Addresses.Swarm '["/ip4/0.0.0.0/tcp/4101","/ip4/0.0.0.0/udp/4101/quic-v1"]'
ipfs config Addresses.API /ip4/127.0.0.1/tcp/5101
ipfs config Addresses.Gateway /ip4/127.0.0.1/tcp/8181
ipfs daemon
```

Target your own node explicitly with `ipfs --api=/ip4/127.0.0.1/tcp/5101 ...`. Shut down only the daemons you started (track their PIDs); do not `pkill` indiscriminately when another node may be running on the host.

### Testing AutoTLS Locally

AutoTLS only requests a `*.libp2p.direct` certificate once libp2p confirms the node is publicly reachable on a TCP port. For a local test the node must be able to open that port, so enable UPnP/NAT-PMP (the `server` init profile disables it via `Swarm.DisableNatPortMap: true`):

```bash
ipfs config --json Swarm.DisableNatPortMap false   # let UPnP/NAT-PMP map the swarm port
ipfs config AutoTLS.RegistrationDelay 5s           # shorten the default wait before registration
```

Then start the daemon and watch the relevant logs:

```bash
GOLOG_LOG_LEVEL="error,autotls=info,nat=info" ipfs daemon
```

Poll `ipfs id` until a `tls/ws` address under your own peer ID appears. A `libp2p.direct` address ending in `/p2p-circuit/p2p/<your-id>` is a relay path, not your own AutoTLS cert. Requires a router that actually honors UPnP/NAT-PMP; without it AutoNAT reports `Private` and no certificate is issued.
