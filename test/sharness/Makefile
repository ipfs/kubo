# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

# NOTE: run with TEST_VERBOSE=1 for verbose sharness tests.

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
BINS = bin/random bin/multihash bin/ipfs bin/pollEndpoint \
	   bin/iptb
SHARNESS = lib/sharness/sharness.sh
IPFS_ROOT = ../..

# User might want to override those on the command line
GOFLAGS =

all: aggregate

clean: clean-test-results
	@echo "*** $@ ***"
	-rm -rf bin/ipfs

clean-test-results:
	@echo "*** $@ ***"
	-rm -rf test-results

$(T): clean-test-results deps
	@echo "*** $@ ***"
	./$@

aggregate: clean-test-results $(T)
	@echo "*** $@ ***"
	lib/test-aggregate-results.sh

deps: $(SHARNESS) $(BINS) curl

$(SHARNESS):
	@echo "*** installing $@ ***"
	lib/install-sharness.sh

bin/%: FORCE
	cd .. && make GOFLAGS=$(GOFLAGS) $@

race:
	make GOFLAGS=-race all

.PHONY: all clean $(T) aggregate FORCE

# will fail if curl is not installed.
# TODO: get rid of this and install curl with git or ipfs.
install_curl_pls = "curl is required to run tests. please install it"
curl:
	@which curl >/dev/null || (echo "$(install_curl_pls)" && false)
