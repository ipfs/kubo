# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

# NOTE: run with TEST_VERBOSE=1 for verbose sharness tests.

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
BINS = bin/random bin/multihash bin/ipfs bin/pollEndpoint
SHARNESS = lib/sharness/sharness.sh
IPFS_ROOT = ../..
HELPER_SOURCES = $(wildcard src/*.go)
HELPER_BINARIES = $(patsubst src/%.go,bin/%,$(HELPER_SOURCES))

all: clean deps $(T) aggregate

clean:
	@echo "*** $@ ***"
	-rm -rf test-results
	-rm -rf bin/ipfs
	-rm -rf $(HELPER_BINARIES)

$(T):
	@echo "*** $@ ***"
	./$@

aggregate:
	@echo "*** $@ ***"
	lib/test-aggregate-results.sh

deps: $(SHARNESS) $(BINS) $(HELPER_BINARIES) curl

$(SHARNESS):
	@echo "*** installing $@ ***"
	lib/install-sharness.sh

bin/%: $(IPFS_ROOT)/**/*.go
	@echo "*** installing $@ ***"
	cd .. && make $@

$(HELPER_BINARIES): bin/%: src/%.go
	@echo "*** installing $@ ***"
	go build -o $@ $^

.PHONY: all clean $(T) aggregate

# will fail if curl is not installed.
# TODO: get rid of this and install curl with git or ipfs.
install_curl_pls = "curl is required to run tests. please install it"
curl:
	@which curl >/dev/null || (echo "$(install_curl_pls)" && false)
