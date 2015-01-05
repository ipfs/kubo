# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

# NOTE: run with TEST_VERBOSE=1 for verbose sharness tests.

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
SHARNESS = lib/sharness/sharness.sh
RANDOMSRC = Godeps/_workspace/src/github.com/jbenet/go-random/random

all: clean deps $(T) aggregate

clean:
	@echo "*** $@ ***"
	-rm -r test-results

$(T):
	@echo "*** $@ ***"
	./$@

aggregate:
	@echo "*** $@ ***"
	lib/test-aggregate-results.sh

deps: $(SHARNESS) ipfs random

$(SHARNESS):
	@echo "*** installing $@ ***"
	lib/install-sharness.sh

# phony to ensure we re-build it every time we run tests
ipfs:
	@echo "*** installing $@ ***"
	mkdir -p bin
	cd ../cmd/ipfs && go build
	cp ../cmd/ipfs/ipfs bin/ipfs

random:
	@echo "*** installing $@ ***"
	mkdir -p bin
	go build -o bin/random ../$(RANDOMSRC)

.PHONY: all clean $(T) aggregate ipfs random
