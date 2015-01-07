
BINS = bin/random bin/ipfs
IPFS_ROOT = ../
IPFS_CMD = ../cmd/ipfs
RANDOM_SRC = ../Godeps/_workspace/src/github.com/jbenet/go-random

all: deps

deps: bins

clean:
	rm $(BINS)

bins: $(BINS)

bin/random: $(RANDOM_SRC)/**/*.go
	go build -o bin/random $(RANDOM_SRC)/random

bin/ipfs: $(IPFS_ROOT)/**/*.go
	go build -o bin/ipfs $(IPFS_CMD)

test: test_expensive

test_expensive:
	cd sharness && make TEST_EXPENSIVE=1
	cd 3nodetest && make

test_cheap:
	cd sharness && make
	cd 3nodetest && make

.PHONY: all clean
