# golang utilities
GO_MIN_VERSION = 1.25
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

test_go_test: $$(DEPS_GO)
	$(GOCC) test $(go-flags-with-tags) $(GOTFLAGS) ./...
.PHONY: test_go_test

# Build all platforms from .github/build-platforms.yml
test_go_build:
	bin/test-go-build-platforms
.PHONY: test_go_build

test_go_short: GOTFLAGS += -test.short
test_go_short: test_go_test
.PHONY: test_go_short

test_go_race: GOTFLAGS += -race
test_go_race: test_go_test
.PHONY: test_go_race

test_go_expensive: test_go_test test_go_build
.PHONY: test_go_expensive
TEST_GO += test_go_expensive

test_go_fmt:
	bin/test-go-fmt
.PHONY: test_go_fmt
TEST_GO += test_go_fmt

test_go_lint: test/bin/golangci-lint
	golangci-lint run --timeout=3m ./...
.PHONY: test_go_lint

test_go: $(TEST_GO)

check_go_version:
	@$(GOCC) version
	bin/check_go_version $(GO_MIN_VERSION)
.PHONY: check_go_version
DEPS_GO += check_go_version

TEST += $(TEST_GO)
TEST_SHORT += test_go_fmt test_go_short
