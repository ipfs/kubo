# golang utilities
export GO111MODULE=on


# pre-definitions
GOCC ?= go
GOTAGS ?=
GOTFLAGS ?=

# Unexport GOFLAGS so we only apply it where we actually want it.
unexport GOFLAGS
# Override so we can combine with the user's go flags.
# Try to make building as reproducible as possible by stripping the go path.
override GOFLAGS += "-trimpath"

ifeq ($(tarball-is),1)
	GOFLAGS += -mod=vendor
endif

# match Go's default GOPATH behaviour
export GOPATH ?= $(shell $(GOCC) env GOPATH)

DEPS_GO :=
TEST_GO :=
TEST_GO_BUILD :=
CHECK_GO :=

go-pkg-name=$(shell GOFLAGS=-buildvcs=false $(GOCC) list $(go-tags) github.com/ipfs/kubo/$(1))
go-main-name=$(notdir $(call go-pkg-name,$(1)))$(?exe)
go-curr-pkg-tgt=$(d)/$(call go-main-name,$(d))
go-pkgs=$(shell GOFLAGS=-buildvcs=false $(GOCC) list github.com/ipfs/kubo/...)

go-tags=$(if $(GOTAGS), -tags="$(call join-with,$(space),$(GOTAGS))")
go-flags-with-tags=$(GOFLAGS)$(go-tags)

define go-build-relative
$(GOCC) build $(go-flags-with-tags) -o "$@" "$(call go-pkg-name,$<)"
endef

define go-build
$(GOCC) build $(go-flags-with-tags) -o "$@" "$(1)"
endef

# Only disable colors when running in CI (non-interactive terminal)
GOTESTSUM_NOCOLOR := $(if $(CI),--no-color,)

# Packages excluded from coverage (test code and examples are not production code)
COVERPKG_EXCLUDE := /(test|docs/examples)/

# Packages excluded from unit tests: coverage exclusions + client/rpc (tested by test_cli)
UNIT_EXCLUDE := /(test|docs/examples)/|/client/rpc$$

# Unit tests with coverage
# Produces JSON for CI reporting and coverage profile for Codecov
test_unit: test/bin/gotestsum $$(DEPS_GO)
	mkdir -p test/unit coverage
	rm -f test/unit/gotest.json coverage/unit_tests.coverprofile
	gotestsum $(GOTESTSUM_NOCOLOR) --jsonfile test/unit/gotest.json -- $(go-flags-with-tags) $(GOTFLAGS) -covermode=atomic -coverprofile=coverage/unit_tests.coverprofile -coverpkg=$$($(GOCC) list $(go-tags) ./... | grep -vE '$(COVERPKG_EXCLUDE)' | tr '\n' ',' | sed 's/,$$//') $$($(GOCC) list $(go-tags) ./... | grep -vE '$(UNIT_EXCLUDE)')
.PHONY: test_unit

# CLI/integration tests (requires built binary in PATH)
# Includes test/cli, test/integration, and client/rpc
# Produces JSON for CI reporting
# Override TEST_CLI_TIMEOUT for local development: make test_cli TEST_CLI_TIMEOUT=5m
TEST_CLI_TIMEOUT ?= 10m
test_cli: cmd/ipfs/ipfs test/bin/gotestsum $$(DEPS_GO)
	mkdir -p test/cli
	rm -f test/cli/cli-tests.json
	PATH="$(CURDIR)/cmd/ipfs:$(CURDIR)/test/bin:$$PATH" gotestsum $(GOTESTSUM_NOCOLOR) --jsonfile test/cli/cli-tests.json -- -v -timeout=$(TEST_CLI_TIMEOUT) ./test/cli/... ./test/integration/... ./client/rpc/...
.PHONY: test_cli

# Example tests (docs/examples/kubo-as-a-library)
# Tests against both published and current kubo versions
# Uses timeout to ensure CI gets output before job-level timeout kills everything
TEST_EXAMPLES_TIMEOUT ?= 2m
test_examples:
	cd docs/examples/kubo-as-a-library && go test -v -timeout=$(TEST_EXAMPLES_TIMEOUT) ./... && cp go.mod go.mod.bak && cp go.sum go.sum.bak && (go mod edit -replace github.com/ipfs/kubo=./../../.. && go mod tidy && go test -v -timeout=$(TEST_EXAMPLES_TIMEOUT) ./...; ret=$$?; mv go.mod.bak go.mod; mv go.sum.bak go.sum; exit $$ret)
.PHONY: test_examples

# Build kubo for all platforms from .github/build-platforms.yml
test_go_build:
	bin/test-go-build-platforms
.PHONY: test_go_build

# Check Go source formatting
test_go_fmt:
	bin/test-go-fmt
.PHONY: test_go_fmt

# Run golangci-lint (used by CI)
test_go_lint: test/bin/golangci-lint
	golangci-lint run --timeout=3m ./...
.PHONY: test_go_lint

TEST_GO := test_go_fmt test_unit test_cli test_examples
TEST += $(TEST_GO)
TEST_SHORT += test_go_fmt test_unit
