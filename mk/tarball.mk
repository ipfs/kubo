

ifeq (,$(wildcard .tarball))
tarball-is:=0
else
tarball-is:=1
# override git hash
git-hash:=$(shell cat .tarball)
endif


go-ipfs-source.tar.gz: distclean
	bin/maketarball.sh $@
