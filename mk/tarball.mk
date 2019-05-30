

ifeq (,$(wildcard .tarball))
tarball-is:=0
else
tarball-is:=1
# override git hash
git-hash:=$(shell cat .tarball)
endif

GOCC ?= go

go-ipfs-source.tar.gz: distclean
	GOCC=$(GOCC) bin/maketarball.sh $@
