# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
SHARNESS = sharness/sharness.sh
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
	./test-aggregate-results.sh

deps: $(SHARNESS) ipfs random

$(SHARNESS):
	@echo "*** installing $@ ***"
	./install-sharness.sh

# phony to ensure we re-build it every time we run tests
ipfs:
	@echo "*** installing $@ ***"
	cd ../cmd/ipfs && go build
	cp ../cmd/ipfs/ipfs ipfs

random:
	@echo "*** installing $@ ***"
	go build ../$(RANDOMSRC)
	cp ../$(RANDOMSRC)/random random

.PHONY: all clean $(T) aggregate ipfs random
