# Minimum version numbers for software required to build IPFS
IPFS_MIN_GO_VERSION = 1.7
IPFS_MIN_GX_VERSION = 0.6
IPFS_MIN_GX_GO_VERSION = 1.1

GOTAGS =
GOTAGS += "" # we have to have always at least one tag, empty tag works well

GOFLAGS =
GOTFLAGS =

export IPFS_REUSEPORT=false
export GOFLAGS
export GOTFLAGS

GOFLAGS += -tags $(call join-with,$(comma),$(GOTAGS))

ifeq ($(TEST_NO_FUSE),1)
	GOTAGS += nofuse
endif

ifeq ($(OS),Windows_NT)
	GOPATH_DELIMITER = ;
else
	GOPATH_DELIMITER = :
endif

dist_root=/ipfs/QmNZL8wNsvAGdVYr8uGeUE9aGfHjFpHegAWywQFEdSaJbp
gx_bin=bin/gx-v0.9.0
gx-go_bin=bin/gx-go-v1.3.0


# util functions

space =
space +=
comma =,
join-with = $(subst $(space),$1,$(strip $2))
# use things in our bin before any other system binaries
export PATH := bin:$(PATH)
export IPFS_API ?= v04x.ipfs.io

all: help

godep:
	go get github.com/tools/godep

go_check:
	@bin/check_go_version $(IPFS_MIN_GO_VERSION)

bin/gx-v%:
	@echo "installing gx $(@:bin/gx-%=%)"
	@bin/dist_get ${dist_root} gx $@ $(@:bin/gx-%=%)
	rm -f bin/gx
	ln -s $(@:bin/%=%) bin/gx

bin/gx-go-v%:
	@echo "installing gx-go $(@:bin/gx-go-%=%)"
	@bin/dist_get ${dist_root} gx-go $@ $(@:bin/gx-go-%=%)
	rm -f bin/gx-go
	ln -s $(@:bin/%=%) bin/gx-go

gx_check: ${gx_bin} ${gx-go_bin}

path_check:
	@bin/check_go_path $(realpath $(shell pwd)) $(realpath $(addsuffix /src/github.com/ipfs/go-ipfs,$(subst $(GOPATH_DELIMITER), ,$(GOPATH))))

deps: go_check gx_check path_check $(covertools_rule)
	${gx_bin} --verbose install --global

deps_covertools:
	go get -u github.com/wadey/gocovmerge
	go get -u golang.org/x/tools/cmd/cover

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

nofuse: GOTAGS += nofuse
nofuse: deps
	$(MAKE) -C cmd/ipfs install

install build: deps
	$(MAKE) -C cmd/ipfs $@

clean:
	$(MAKE) -C cmd/ipfs clean
	$(MAKE) -C test clean

uninstall:
	$(MAKE) -C cmd/ipfs uninstall

PHONY += all help godep gx_check covertools
PHONY += go_check deps vendor install build nofuse clean uninstall

##############################################################
# tests targets

test: test_expensive

test_short: test_go_fmt build test_go_short test_sharness_short

test_expensive: test_go_fmt build test_go_expensive test_sharness_expensive windows_build_check

test_3node:
	$(MAKE) -C test/3nodetest

test_go_fmt:
	bin/test-go-fmt

test_go_short: GOTFLAGS += -test.short
test_go_race: GOTFLAGS += -race
test_go_expensive test_go_short test_go_race:
	go test $(GOFLAGS) $(GOTFLAGS) ./...

coverage: deps_covertools
	@echo Running coverage
	$(eval PKGS := $(shell go list -f '{{if (len .GoFiles)}}{{.ImportPath}}{{end}}' ./... | grep -v /vendor/ | grep -v /Godeps/))
#$(eval PKGS_DELIM := $(call join-with,$(comma),$(PKGS)))
	@go list -f '{{if or (len .TestGoFiles) (len .XTestGoFiles)}}go test $(GOFLAGS) $(GOTFLAGS) -covermode=atomic -coverprofile={{.Name}}_{{len .Imports}}_{{len .Deps}}.coverprofile {{.ImportPath}}{{end}}' $(GOFLAGS) $(PKGS) | xargs -I {} bash -c {} 2>&1 | grep -v 'warning: no packages being tested depend on'
	gocovmerge `ls *.coverprofile` > coverage.txt
	rm *.coverprofile
	bash -c 'bash <(curl -s https://codecov.io/bash)'

test_sharness_short:
	$(MAKE) -j1 -C test/sharness/

test_sharness_expensive:
	TEST_EXPENSIVE=1 $(MAKE) -j1 -C test/sharness/

test_all_commits:
	@echo "testing all commits between origin/master..HEAD"
	@echo "WARNING: this will 'git rebase --exec'."
	@test/bin/continueyn
	GIT_EDITOR=true git rebase -i --exec "$(MAKE) test" origin/master

test_all_commits_travis:
	# these are needed because travis.
	# we don't use this yet because it takes way too long.
	git config --global user.email "nemo@ipfs.io"
	git config --global user.name "IPFS BOT"
	git fetch origin master:master
	GIT_EDITOR=true git rebase -i --exec "$(MAKE) test" master

# since we have CI for osx and linux but not windows, this should help
windows_build_check:
	GOOS=windows GOARCH=amd64 go build -o .test.ipfs.exe ./cmd/ipfs
	rm -f .test.ipfs.exe

PHONY += test test_short test_expensive

##############################################################
# A semi-helpful help message

help:
	@echo 'DEPENDENCY TARGETS:'
	@echo ''
	@echo '  gx_check        - Installs or upgrades gx and gx-go'
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
