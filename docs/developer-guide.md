# Developer Guide

By the end of this guide, you will be able to:

- Build Kubo from source
- Run the test suites
- Make and verify code changes

This guide covers the local development workflow. For user documentation, see [docs.ipfs.tech](https://docs.ipfs.tech/).

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Building](#building)
- [Running Tests](#running-tests)
- [Running the Linter](#running-the-linter)
- [Common Development Tasks](#common-development-tasks)
- [Code Organization](#code-organization)
- [Architecture](#architecture)
- [Troubleshooting](#troubleshooting)
- [Development Dependencies](#development-dependencies)
- [Further Reading](#further-reading)

## Prerequisites

Before you begin, ensure you have:

- **Go** - see `go.mod` for the minimum required version
- **Git**
- **GNU Make**
- **GCC** (optional) - required for CGO (Go's C interop); without it, build with `CGO_ENABLED=0`

## Quick Start

```bash
git clone https://github.com/ipfs/kubo.git
cd kubo
make build
./cmd/ipfs/ipfs version
```

You should see output like:

```
ipfs version 0.34.0-dev
```

The binary is built to `cmd/ipfs/ipfs`. To install it system-wide:

```bash
make install
```

This installs the binary to `$GOPATH/bin`.

## Building

| Command | Description |
|---------|-------------|
| `make build` | build the `ipfs` binary to `cmd/ipfs/ipfs` |
| `make install` | install to `$GOPATH/bin` |
| `make nofuse` | build without FUSE support |
| `make build CGO_ENABLED=0` | build without CGO (no C compiler needed) |

For Windows-specific instructions, see [windows.md](windows.md).

## Running Tests

Kubo has two types of tests:

- **Unit tests** - test individual packages in isolation. Fast and don't require a running daemon.
- **End-to-end tests** - spawn real `ipfs` nodes, run actual CLI commands, and test the full system. Slower but catch integration issues.

Note that `go test ./...` runs both unit and end-to-end tests. Use `make test` to run all tests. CI runs unit and end-to-end tests in separate jobs for faster feedback.

<!-- TODO: uncomment when https://github.com/ipfs/kubo/pull/11113 is merged
| Command | What it runs |
|---------|--------------|
| `make test_unit` | unit tests only (excludes `test/cli`) |
| `make test_cli` | CLI end-to-end tests only (requires `make build` first) |
| `make test_sharness` | sharness end-to-end tests only |
| `make test` | all tests (unit + CLI + sharness) |
-->

For end-to-end tests, Kubo has two suites:

- **`test/cli`** - modern Go-based test harness that spawns real `ipfs` nodes and runs actual CLI commands. All new tests should be added here.
- **`test/sharness`** - legacy bash-based tests. We are slowly migrating these to `test/cli`.

When modifying tests: cosmetic changes to `test/sharness` are fine, but if significant rewrites are needed, remove the outdated sharness test and add a modern one to `test/cli` instead.

### Before Running Tests

**Environment requirements**: some legacy tests expect default ports (8080, 5001, 4001) to be free and no mDNS (local network discovery) Kubo service on the LAN. Tests may fail if you have a local Kubo instance running. Before running the full test suite, stop any running `ipfs daemon`.

Two critical setup steps:

1. **Rebuild after code changes**: if you modify any `.go` files outside of `test/`, you must run `make build` before running integration tests.

2. **Set environment variables**: integration tests use the `ipfs` binary from `PATH` and need an isolated `IPFS_PATH`. Run these commands from the repository root:

```bash
export PATH="$PWD/cmd/ipfs:$PATH"
export IPFS_PATH="$(mktemp -d)"
```

### Unit Tests

```bash
go test ./...
```

### CLI Integration Tests (`test/cli`)

These are Go-based integration tests that invoke the `ipfs` CLI.

Instead of running the entire test suite, you can run a specific test to get faster feedback during development.

Run a specific test (recommended during development):

```bash
go test ./test/cli/... -run TestAdd -v
```

Run all CLI tests:

```bash
go test ./test/cli/...
```

Run a specific test:

```bash
go test ./test/cli/... -run TestAdd
```

Run with verbose output:

```bash
go test ./test/cli/... -v
```

**Common error**: "version (16) is lower than repos (17)" means your `PATH` points to an old binary. Check `which ipfs` and rebuild with `make build`.

### Sharness Tests (`test/sharness`)

Shell-based integration tests using [sharness](https://github.com/chriscool/sharness) (a portable shell testing framework).

```bash
cd test/sharness
```

Run a specific test:

```bash
timeout 60s ./t0080-repo.sh
```

Run with verbose output (this disables automatic cleanup):

```bash
./t0080-repo.sh -v
```

**Cleanup**: the `-v` flag disables automatic cleanup. Before re-running tests, kill any dangling `ipfs daemon` processes:

```bash
pkill -f "ipfs daemon"
```

### Full Test Suite

```bash
make test        # run all tests
make test_short  # run shorter test suite
```

## Running the Linter

Run the linter using the Makefile target (not `golangci-lint` directly):

```bash
make -O test_go_lint
```

## Common Development Tasks

### Modifying CLI Commands

After editing help text in `core/commands/`, verify the output width:

```bash
go test ./test/cli/... -run TestCommandDocsWidth
```

### Updating Dependencies

Use the Makefile target (not `go mod tidy` directly):

```bash
make mod_tidy
```

### Editing the Changelog

When modifying `docs/changelogs/`:

- update the Table of Contents when adding sections
- add user-facing changes to the Highlights section (the Changelog section is auto-generated)

### Running the Daemon

Always run the daemon with a timeout or shut it down promptly.

With timeout:

```bash
timeout 60s ipfs daemon
```

Or shut down via API:

```bash
ipfs shutdown
```

For multi-step experiments, store `IPFS_PATH` in a file to ensure consistency.

## Code Organization

| Directory | Description |
|-----------|-------------|
| `cmd/ipfs/` | CLI entry point and binary |
| `core/` | core IPFS node implementation |
| `core/commands/` | CLI command definitions |
| `core/coreapi/` | Go API implementation |
| `client/rpc/` | HTTP RPC client |
| `plugin/` | plugin system |
| `repo/` | repository management |
| `test/cli/` | Go-based CLI integration tests |
| `test/sharness/` | legacy shell-based integration tests |
| `docs/` | documentation |

Key external dependencies:

- [go-libp2p](https://github.com/libp2p/go-libp2p) - networking stack
- [go-libp2p-kad-dht](https://github.com/libp2p/go-libp2p-kad-dht) - distributed hash table
- [boxo](https://github.com/ipfs/boxo) - IPFS SDK (including Bitswap, the data exchange engine)

For a deep dive into how code flows through Kubo, see [The `Add` command demystified](add-code-flow.md).

## Architecture

**Map of Implemented Subsystems** ([editable source](https://docs.google.com/drawings/d/1OVpBT2q-NtSJqlPX3buvjYhOnWfdzb85YEsM_njesME/edit)):

<img src="https://docs.google.com/drawings/d/e/2PACX-1vS_n1FvSu6mdmSirkBrIIEib2gqhgtatD9awaP2_WdrGN4zTNeg620XQd9P95WT-IvognSxIIdCM5uE/pub?w=1446&amp;h=1036">

**CLI, HTTP-API, Core Diagram**:

![](./cli-http-api-core-diagram.png)

## Troubleshooting

### "version (N) is lower than repos (M)" Error

This means the `ipfs` binary in your `PATH` is older than expected.

Check which binary is being used:

```bash
which ipfs
```

Rebuild and verify PATH:

```bash
make build
export PATH="$PWD/cmd/ipfs:$PATH"
./cmd/ipfs/ipfs version
```

### FUSE Issues

If you don't need FUSE support, build without it:

```bash
make nofuse
```

Or set the `TEST_FUSE=0` environment variable when running tests.

### Build Fails with "No such file: stdlib.h"

You're missing a C compiler. Either install GCC or build without CGO:

```bash
make build CGO_ENABLED=0
```

## Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://github.com/google/protobuf).

## Further Reading

- [The `Add` command demystified](add-code-flow.md) - deep dive into code flow
- [Configuration reference](config.md)
- [Performance debugging](debug-guide.md)
- [Experimental features](experimental-features.md)
- [Release process](releases.md)
- [Contributing guidelines](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

## Source Code

The complete source code is at [github.com/ipfs/kubo](https://github.com/ipfs/kubo).
