include mk/header.mk

TGTS_$(d) :=

$(d)/pollEndpoint: thirdparty/pollEndpoint
	$(go-build-relative)
TGTS_$(d) += $(d)/pollEndpoint

$(d)/go-sleep: test/dependencies/go-sleep
	$(go-build-relative)
TGTS_$(d) += $(d)/go-sleep

$(d)/go-timeout: test/dependencies/go-timeout
	$(go-build-relative)
TGTS_$(d) += $(d)/go-timeout

$(d)/iptb: test/dependencies/iptb
	$(go-build-relative)
TGTS_$(d) += $(d)/iptb

$(d)/ma-pipe-unidir: test/dependencies/ma-pipe-unidir
	$(go-build-relative)
TGTS_$(d) += $(d)/ma-pipe-unidir

$(d)/json-to-junit: test/dependencies/json-to-junit
	$(go-build-relative)
TGTS_$(d) += $(d)/json-to-junit

$(d)/hang-fds:
	$(call go-build,github.com/ipfs/hang-fds)
TGTS_$(d) += $(d)/hang-fds

$(d)/multihash:
	$(call go-build,github.com/multiformats/go-multihash/multihash)
TGTS_$(d) += $(d)/multihash

$(d)/cid-fmt:
	$(call go-build,github.com/ipfs/go-cidutil/cid-fmt)
TGTS_$(d) += $(d)/cid-fmt

$(d)/random:
	$(call go-build,github.com/jbenet/go-random/random)
TGTS_$(d) += $(d)/random

$(d)/random-files:
	$(call go-build,github.com/jbenet/go-random-files/random-files)
TGTS_$(d) += $(d)/random-files


$(TGTS_$(d)): $$(DEPS_GO)

CLEAN += $(TGTS_$(d))

PATH := $(realpath $(d)):$(PATH)

include mk/footer.mk
