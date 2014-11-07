# Run tests
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

T = $(sort $(wildcard t[0-9][0-9][0-9][0-9]-*.sh))
RANDOM=$(GOPATH)/bin/random

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

deps: sharness.sh $(RANDOM)

sharness.sh:
	@echo "*** installing $@ ***"
	./install-sharness.sh

.PHONY: all clean $(T) aggregate

$(RANDOM):
	go get github.com/jbenet/go-random/random
	go install github.com/jbenet/go-random/random
