
BINS = bin/random bin/multihash bin/ipfs bin/pollEndpoint bin/iptb bin/go-sleep
BINS += bin/go-timeout
IPFS_ROOT = ../
IPFS_CMD = ../cmd/ipfs
RANDOM_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-random
RANDOM_FILES_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-random-files
POLLENDPOINT_SRC= ../thirdparty/pollEndpoint
GOSLEEP_SRC = ./dependencies/go-sleep
GOTIMEOUT_SRC = ./dependencies/go-timeout

export PATH := ../bin:${PATH}

# User might want to override those on the command line
GOFLAGS =

all: deps

global-deps:
	$(MAKE) -C .. deps

deps: global-deps bins

clean:
	$(MAKE) -C sharness clean
	rm -rf $(BINS)

bins: $(BINS)

find_go_files = $(shell find $(1) -name "*.go")

# Non gx dependencies

bin/random: $(call find_go_files, $(RANDOM_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/random $(RANDOM_SRC)/random

bin/random-files:
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/random-files $(RANDOM_FILES_SRC)/random-files

bin/ipfs: $(call find_go_files, $(IPFS_ROOT)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/ipfs $(IPFS_CMD)

bin/pollEndpoint: $(call find_go_files, $(POLLENDPOINT_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/pollEndpoint $(POLLENDPOINT_SRC)

bin/go-sleep: $(call find_go_files, $(GOSLEEP_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/go-sleep $(GOSLEEP_SRC)

bin/go-timeout: $(call find_go_files, $(GOTIMEOUT_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/go-timeout $(GOTIMEOUT_SRC)

# gx dependencies

multihash_src:
	$(eval MULTIHASH_HASH := $(shell cd .. && bin/gx deps find go-multihash))
	$(eval MULTIHASH_SRC := gx/ipfs/$(MULTIHASH_HASH)/go-multihash)

hang-fds_src:
	$(eval HANG_FDS_HASH := $(shell cd .. && bin/gx deps find hang-fds))
	$(eval HANG_FDS_SRC := gx/ipfs/$(HANG_FDS_HASH)/hang-fds)

bin/multihash: multihash_src $(call find_go_files, $(MULTIHASH_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/multihash $(MULTIHASH_SRC)/multihash

bin/hang-fds: hang-fds_src $(call find_go_files, $(HANG_FDS_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/hang-fds $(HANG_FDS_SRC)

iptb_src:
	$(eval IPTB_HASH := $(shell cd .. && bin/gx deps find iptb))
	$(eval IPTB_SRC := gx/ipfs/$(IPTB_HASH)/iptb)

bin/iptb: iptb_src $(call find_go_files, $(IPTB_SRC)) IPFS-BUILD-OPTIONS
	@echo "*** installing $@ ***"
	go build $(GOFLAGS) -o bin/iptb $(IPTB_SRC)

# Tests

test: test_expensive

test_expensive: verify_gofmt
	$(MAKE) -C sharness TEST_EXPENSIVE=1
	$(MAKE) -C 3nodetest
	$(MAKE) -C dependencies

test_cheap: verify_gofmt
	$(MAKE) -C sharness
	$(MAKE) -C 3nodetest

test_race: verify_gofmt
	$(MAKE) -C sharness GOFLAGS=-race TEST_EXPENSIVE=1
	$(MAKE) -C 3nodetest GOFLAGS=-race
	$(MAKE) -C dependencies GOFLAGS=-race

# Misc

coverage: coverage_sharness

coverage_sharness:
	./sharness_test_coverage_helper.sh

IPFS-BUILD-OPTIONS: FORCE
	@bin/checkflags '$@' '$(GOFLAGS)' '*** new Go flags ***'

verify_gofmt:
	bin/verify-go-fmt.sh

.PHONY: all clean FORCE
