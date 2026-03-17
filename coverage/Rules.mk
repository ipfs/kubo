include mk/header.mk

GOCC ?= go

$(d)/coverage_deps: $$(DEPS_GO) cmd/ipfs/ipfs
	rm -rf $(@D)/sharnesscover && mkdir $(@D)/sharnesscover

.PHONY: $(d)/coverage_deps

# unit tests coverage is now produced by test_unit target in mk/golang.mk
# (outputs coverage/unit_tests.coverprofile and test/unit/gotest.json)

TGTS_$(d) :=

# sharness tests coverage
$(d)/ipfs: GOTAGS += testrunmain
$(d)/ipfs: $(d)/main
	$(go-build-relative)

CLEAN += $(d)/ipfs

ifneq ($(filter coverage%,$(MAKECMDGOALS)),)
	# this is quite hacky but it is best way I could figure out
	DEPS_test/sharness += cmd/ipfs/ipfs-test-cover $(d)/coverage_deps $(d)/ipfs
endif

export IPFS_COVER_DIR:= $(realpath $(d))/sharnesscover/

$(d)/sharness_tests.coverprofile: export TEST_PLUGIN=0
$(d)/sharness_tests.coverprofile: $(d)/ipfs cmd/ipfs/ipfs-test-cover $(d)/coverage_deps test/bin/gocovmerge test_sharness
	(cd $(@D)/sharnesscover && find . -type f | gocovmerge -list -) > $@


PATH := $(realpath $(d)):$(PATH)

TGTS_$(d) += $(d)/sharness_tests.coverprofile

CLEAN += $(TGTS_$(d))
COVERAGE += $(TGTS_$(d))

include mk/footer.mk
