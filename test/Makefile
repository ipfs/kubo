
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

test_race:
	cd sharness && make GOFLAGS=-race TEST_EXPENSIVE=1
	cd 3nodetest && make GOFLAGS=-race
	cd dependencies && make GOFLAGS=-race

IPFS-BUILD-OPTIONS: FORCE
	@bin/checkflags '$@' '$(GOFLAGS)' '*** new Go flags ***'

.PHONY: all clean FORCE
