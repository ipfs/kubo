
BINS = bin/random bin/multihash bin/ipfs bin/pollEndpoint
IPFS_ROOT = ../
IPFS_CMD = ../cmd/ipfs
RANDOM_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-random
MULTIHASH_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-multihash
POLLENDPOINT_SRC= ../thirdparty/pollEndpoint

# User might want to override those on the command line
GOFLAGS =

all: deps

deps: bins

clean:
	rm $(BINS)

bins: $(BINS)

bin/random: $(RANDOM_SRC)/**/*.go IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/random $(RANDOM_SRC)/random

bin/multihash: $(MULTIHASH_SRC)/**/*.go IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/multihash $(MULTIHASH_SRC)/multihash

bin/ipfs: $(IPFS_ROOT)/**/*.go IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/ipfs $(IPFS_CMD)

bin/pollEndpoint: $(POLLENDPOINT_SRC)/*.go IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/pollEndpoint $(POLLENDPOINT_SRC)

test: test_expensive

test_expensive:
	cd sharness && make TEST_EXPENSIVE=1
	cd 3nodetest && make
	cd dependencies && make

test_cheap:
	cd sharness && make
	cd 3nodetest && make

IPFS-BUILD-OPTIONS: FORCE
	@VARS='$(GOFLAGS)'; \
	    if test x"$$VARS" != x"`cat $@ 2>/dev/null`" ; then \
		echo "*** new Go flags ***"; \
		echo "$$VARS" >$@; \
            fi

.PHONY: all clean FORCE
