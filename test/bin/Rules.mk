include mk/header.mk

TGTS_$(d) :=

define go-build-testdep
	OUT="$(CURDIR)/$@" ; \
	cd "test/dependencies" ; \
	$(GOCC) build $(go-flags-with-tags) -o "$${OUT}" "$<" 2>&1
endef

.PHONY: github.com/ipfs/kubo/test/dependencies/pollEndpoint
$(d)/pollEndpoint: github.com/ipfs/kubo/test/dependencies/pollEndpoint
	$(go-build-testdep)
TGTS_$(d) += $(d)/pollEndpoint

.PHONY: github.com/ipfs/kubo/test/dependencies/go-sleep
$(d)/go-sleep: github.com/ipfs/kubo/test/dependencies/go-sleep
	$(go-build-testdep)
TGTS_$(d) += $(d)/go-sleep

.PHONY: github.com/ipfs/kubo/test/dependencies/go-timeout
$(d)/go-timeout: github.com/ipfs/kubo/test/dependencies/go-timeout
	$(go-build-testdep)
TGTS_$(d) += $(d)/go-timeout

.PHONY: github.com/ipfs/kubo/test/dependencies/iptb
$(d)/iptb: github.com/ipfs/kubo/test/dependencies/iptb
	$(go-build-testdep)
TGTS_$(d) += $(d)/iptb

.PHONY: github.com/ipfs/kubo/test/dependencies/ma-pipe-unidir
$(d)/ma-pipe-unidir: github.com/ipfs/kubo/test/dependencies/ma-pipe-unidir
	$(go-build-testdep)
TGTS_$(d) += $(d)/ma-pipe-unidir

.PHONY: github.com/ipfs/kubo/test/dependencies/json-to-junit
$(d)/json-to-junit: github.com/ipfs/kubo/test/dependencies/json-to-junit
	$(go-build-testdep)
TGTS_$(d) += $(d)/json-to-junit

.PHONY: gotest.tools/gotestsum
$(d)/gotestsum: gotest.tools/gotestsum
	$(go-build-testdep)
TGTS_$(d) += $(d)/gotestsum

.PHONY: github.com/ipfs/hang-fds
$(d)/hang-fds: github.com/ipfs/hang-fds
	$(go-build-testdep)
TGTS_$(d) += $(d)/hang-fds

.PHONY: github.com/multiformats/go-multihash/multihash
$(d)/multihash: github.com/multiformats/go-multihash/multihash
	$(go-build-testdep)
TGTS_$(d) += $(d)/multihash

.PHONY: github.com/ipfs/go-cidutil/cid-fmt
$(d)/cid-fmt: github.com/ipfs/go-cidutil/cid-fmt
	$(go-build-testdep)
TGTS_$(d) += $(d)/cid-fmt

.PHONY: github.com/jbenet/go-random/random
$(d)/random: github.com/jbenet/go-random/random
	$(go-build-testdep)
TGTS_$(d) += $(d)/random

.PHONY: github.com/jbenet/go-random-files/random-files
$(d)/random-files: github.com/jbenet/go-random-files/random-files
	$(go-build-testdep)
TGTS_$(d) += $(d)/random-files

.PHONY: github.com/Kubuxu/gocovmerge
$(d)/gocovmerge: github.com/Kubuxu/gocovmerge
	$(go-build-testdep)
TGTS_$(d) += $(d)/gocovmerge

.PHONY: github.com/golangci/golangci-lint/cmd/golangci-lint
$(d)/golangci-lint: github.com/golangci/golangci-lint/cmd/golangci-lint
	$(go-build-testdep)
TGTS_$(d) += $(d)/golangci-lint

$(TGTS_$(d)): $$(DEPS_GO)

CLEAN += $(TGTS_$(d))

PATH := $(realpath $(d)):$(PATH)

include mk/footer.mk
