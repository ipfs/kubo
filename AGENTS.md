# AI Agent Instructions for Kubo

This file provides instructions for AI coding agents working on the [Kubo](https://github.com/ipfs/kubo) codebase (the Go implementation of IPFS). Follow the [Developer Guide](docs/developer-guide.md) for full details.

## Quick Reference

| Task              | Command                                                  |
|-------------------|----------------------------------------------------------|
| Tidy deps         | `make mod_tidy` (run first if `go.mod` changed)         |
| Build             | `make build`                                             |
| Unit tests        | `go test ./... -run TestName -v`                         |
| Integration tests | `make build && go test ./test/cli/... -run TestName -v`  |
| Lint              | `make -O test_go_lint`                                   |
| Format            | `go fmt ./...`                                           |

## Project Overview

Kubo is the reference implementation of IPFS in Go. Most IPFS protocol logic lives in [boxo](https://github.com/ipfs/boxo) (the IPFS SDK); kubo wires it together and exposes it via CLI and HTTP RPC API. If a change belongs in the protocol layer, it likely belongs in boxo, not here.

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

If you modify `go.mod` (add/remove/update dependencies), you must run `make mod_tidy` first, before building or testing. Use `make mod_tidy` instead of `go mod tidy` directly, as the project has multiple `go.mod` files.

If you modify any `.go` files outside of `test/`, you must run `make build` before running integration tests.

## Testing

The full test suite is composed of several targets:

| Make target          | What it runs                                                          |
|----------------------|-----------------------------------------------------------------------|
| `make test`          | all tests (`test_go_fmt` + `test_unit` + `test_cli` + `test_sharness`) |
| `make test_short`    | fast subset (`test_go_fmt` + `test_unit`)                             |
| `make test_unit`     | unit tests with coverage (excludes `test/cli`)                        |
| `make test_cli`      | CLI integration tests (requires `make build` first)                   |
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
- for Go 1.25+, use `testing/synctest` when testing concurrent code (goroutines, channels, timers)
- reuse existing `.car` fixtures in `test/cli/fixtures/` when possible; only add new fixtures when the test requires data not covered by existing ones
- always re-run modified tests locally before submitting to confirm they pass
- avoid emojis in test names and test log output

## Before Submitting

Run these steps in order before considering work complete:

1. `make mod_tidy` (if `go.mod` changed)
2. `go fmt ./...`
3. `make build` (if non-test `.go` files changed)
4. `make -O test_go_lint`
5. `go test ./...` (or the relevant subset)

## Documentation and Commit Messages

- after editing CLI help text in `core/commands/`, verify width: `go test ./test/cli/... -run TestCommandDocsWidth`
- config options are documented in `docs/config.md`
- changelogs in `docs/changelogs/`: only edit the Table of Contents and the Highlights section; the Changelog and Contributors sections are auto-generated and must not be modified
- follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/)
- keep commit titles short and messages terse

## Writing Style

When writing docs, comments, and commit messages:

- avoid emojis in code, comments, and log output
- keep an empty line before lists in markdown
- use backticks around CLI commands, paths, environment variables, and config options

## PR Guidelines

- explain what changed and why in the PR description
- include test coverage for new functionality and bug fixes
- run `make -O test_go_lint` and fix any lint issues before submitting
- verify that `go test ./...` passes locally
- when modifying `test/sharness` tests significantly, migrate them to `test/cli` instead
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
