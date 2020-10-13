include mk/header.mk

GOCC ?= go

$(d)/coverage_deps: $$(DEPS_GO)
	rm -rf $(@D)/unitcover && mkdir $(@D)/unitcover
	rm -rf $(@D)/sharnesscover && mkdir $(@D)/sharnesscover

ifneq ($(IPFS_SKIP_COVER_BINS),1)
$(d)/coverage_deps: test/bin/gocovmerge
endif

.PHONY: $(d)/coverage_deps

# unit tests coverage
UTESTS_$(d) := $(shell $(GOCC) list -f '{{if (or (len .TestGoFiles) (len .XTestGoFiles))}}{{.ImportPath}}{{end}}' $(go-flags-with-tags) ./... | grep -v go-ipfs/vendor | grep -v go-ipfs/Godeps)

UCOVER_$(d) := $(addsuffix .coverprofile,$(addprefix $(d)/unitcover/, $(subst /,_,$(UTESTS_$(d)))))

$(UCOVER_$(d)): $(d)/coverage_deps ALWAYS
	$(eval TMP_PKG := $(subst _,/,$(basename $(@F))))
	$(eval TMP_DEPS := $(shell $(GOCC) list -f '{{range .Deps}}{{.}} {{end}}' $(go-flags-with-tags) $(TMP_PKG) | sed 's/ /\n/g' | grep ipfs/go-ipfs) $(TMP_PKG))
	$(eval TMP_DEPS_LIST := $(call join-with,$(comma),$(TMP_DEPS)))
	$(GOCC) test $(go-flags-with-tags) $(GOTFLAGS) -v -covermode=atomic -json -coverpkg=$(TMP_DEPS_LIST) -coverprofile=$@ $(TMP_PKG) | tee -a test/unit/gotest.json


$(d)/unit_tests.coverprofile: $(UCOVER_$(d))
	gocovmerge $^ > $@

TGTS_$(d) := $(d)/unit_tests.coverprofile

.PHONY: $(d)/unit_tests.coverprofile

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

$(d)/sharness_tests.coverprofile: export TEST_NO_PLUGIN=1
$(d)/sharness_tests.coverprofile: $(d)/ipfs cmd/ipfs/ipfs-test-cover $(d)/coverage_deps test_sharness_short
	(cd $(@D)/sharnesscover && find . -type f | gocovmerge -list -) > $@


PATH := $(realpath $(d)):$(PATH)

TGTS_$(d) += $(d)/sharness_tests.coverprofile

CLEAN += $(TGTS_$(d))
COVERAGE += $(TGTS_$(d))

include mk/footer.mk
