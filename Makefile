# Minimum version numbers for software required to build IPFS
IPFS_MIN_GO_VERSION = 1.5.2
IPFS_MIN_GX_VERSION = 0.6
IPFS_MIN_GX_GO_VERSION = 1.1

ifeq ($(TEST_NO_FUSE),1)
  go_test=go test -tags nofuse
else
  go_test=go test
endif

export IPFS_API ?= v04x.ipfs.io

all: help

godep:
	go get github.com/tools/godep

toolkit_upgrade: gx_upgrade gxgo_upgrade

go_check:
	@bin/check_go_version $(IPFS_MIN_GO_VERSION)

gx_upgrade:
	go get -u github.com/whyrusleeping/gx

gxgo_upgrade:
	go get -u github.com/whyrusleeping/gx-go

path_check:
	@bin/check_go_path $(realpath $(shell pwd)) $(realpath $(GOPATH)/src/github.com/ipfs/go-ipfs)

gx_check:
	@bin/check_gx_program "gx" $(IPFS_MIN_GX_VERSION) 'Upgrade or install gx using your package manager or run `make gx_upgrade`'
	@bin/check_gx_program "gx-go" $(IPFS_MIN_GX_GO_VERSION) 'Upgrade or install gx-go using your package manager or run `make gxgo_upgrade`'

deps: go_check gx_check path_check
	gx --verbose install --global

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

install: deps
	make -C cmd/ipfs install

build: deps
	make -C cmd/ipfs build

nofuse: deps
	make -C cmd/ipfs nofuse

clean:
	make -C cmd/ipfs clean

uninstall:
	make -C cmd/ipfs uninstall

PHONY += all help godep toolkit_upgrade gx_upgrade gxgo_upgrade gx_check
PHONY += go_check deps vendor install build nofuse clean uninstall

##############################################################
# tests targets

test: test_expensive

test_short: build test_go_short test_sharness_short

test_expensive: build test_go_expensive test_sharness_expensive windows_build_check

test_3node:
	cd test/3nodetest && make

test_go_short:
	$(go_test) -test.short ./...

test_go_expensive:
	$(go_test) ./...

test_go_race:
	$(go_test) ./... -race

test_sharness_short:
	cd test/sharness/ && make

test_sharness_expensive:
	cd test/sharness/ && TEST_EXPENSIVE=1 make

test_all_commits:
	@echo "testing all commits between origin/master..HEAD"
	@echo "WARNING: this will 'git rebase --exec'."
	@test/bin/continueyn
	GIT_EDITOR=true git rebase -i --exec "make test" origin/master

test_all_commits_travis:
	# these are needed because travis.
	# we don't use this yet because it takes way too long.
	git config --global user.email "nemo@ipfs.io"
	git config --global user.name "IPFS BOT"
	git fetch origin master:master
	GIT_EDITOR=true git rebase -i --exec "make test" master

# since we have CI for osx and linux but not windows, this should help
windows_build_check:
	GOOS=windows GOARCH=amd64 go build -o .test.ipfs.exe ./cmd/ipfs
	rm .test.ipfs.exe

PHONY += test test_short test_expensive

##############################################################
# A semi-helpful help message

help:
	@echo 'DEPENDENCY TARGETS:'
	@echo ''
	@echo '  toolkit_upgrade - Installs or upgrades gx'
	@echo '  deps            - Download dependencies using gx'
	@echo '  vendor          - Create a Godep workspace of 3rd party dependencies'
	@echo ''
	@echo 'BUILD TARGETS:'
	@echo ''
	@echo '  all          - print this help message'
	@echo '  build        - Build binary at ./cmd/ipfs/ipfs'
	@echo '  nofuse       - Build binary with no fuse support'
	@echo '  install      - Build binary and install into $$GOPATH/bin'
#	@echo '  dist_install - TODO: c.f. ./cmd/ipfs/dist/README.md'
	@echo ''
	@echo 'CLEANING TARGETS:'
	@echo ''
	@echo '  clean        - Remove binary from build directory'
	@echo '  uninstall    - Remove binary from $$GOPATH/bin'
	@echo ''
	@echo 'TESTING TARGETS:'
	@echo ''
	@echo '  test                    - Run expensive tests and Window$$ check'
	@echo '  test_short              - Run short tests and sharness tests'
	@echo '  test_expensive          - Run a few extras'
	@echo '  test_3node'
	@echo '  test_go_short'
	@echo '  test_go_expensive'
	@echo '  test_go_race'
	@echo '  test_sharness_short'
	@echo '  test_sharness_expensive'
	@echo '  test_all_commits'
	@echo "  test_all_commits_travis - DON'T USE: takes way too long"
	@echo '  windows_build_check'
	@echo ''

PHONY += help

.PHONY: $(PHONY)
