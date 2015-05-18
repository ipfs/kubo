
BINS = bin/random bin/multihash bin/ipfs bin/pollEndpoint bin/iptb
IPFS_ROOT = ../
IPFS_CMD = ../cmd/ipfs
RANDOM_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-random
MULTIHASH_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-multihash
IPTB_SRC = ../Godeps/_workspace/src/github.com/whyrusleeping/iptb
POLLENDPOINT_SRC= ../thirdparty/pollEndpoint

# User might want to override those on the command line
GOFLAGS =

all: deps

deps: bins

clean:
	rm $(BINS)

bins: $(BINS)

find_go_files = $(shell find $(1) -name "*.go")

bin/random: $(call find_go_files, $(RANDOM_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/random $(RANDOM_SRC)/random

bin/multihash: $(call find_go_files, $(MULTIHASH_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/multihash $(MULTIHASH_SRC)/multihash

bin/ipfs: $(call find_go_files, $(IPFS_ROOT)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/ipfs $(IPFS_CMD)

bin/pollEndpoint: $(call find_go_files, $(POLLENDPOINT_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/pollEndpoint $(POLLENDPOINT_SRC)

bin/iptb: $(call find_go_files, $(IPTB_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/iptb $(IPTB_SRC)

test: test_expensive

test_expensive: verify_gofmt
	cd sharness && make TEST_EXPENSIVE=1
	cd 3nodetest && make
	cd dependencies && make

test_cheap: verify_gofmt
	cd sharness && make
	cd 3nodetest && make

test_race: verify_gofmt
	cd sharness && make GOFLAGS=-race TEST_EXPENSIVE=1
	cd 3nodetest && make GOFLAGS=-race
	cd dependencies && make GOFLAGS=-race

IPFS-BUILD-OPTIONS: FORCE
	@bin/checkflags '$@' '$(GOFLAGS)' '*** new Go flags ***'

verify_gofmt:
	bin/verify-go-fmt.sh

.PHONY: all clean FORCE
