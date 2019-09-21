include mk/header.mk

dist_root_$(d)="/ipfs/QmPrXH9jRVwvd7r5MC5e6nV4uauQGzLk1i2647Ye9Vbbwe"

TGTS_$(d) := $(d)/protoc
DISTCLEAN += $(d)/protoc $(d)/tmp

PATH := $(realpath $(d)):$(PATH)

$(TGTS_$(d)):
	rm -f $@$(?exe)
ifeq ($(WINDOWS),1)
	cp $^$(?exe) $@$(?exe)
else
	ln -s $(notdir $^) $@
endif

bin/protoc-gen-gogofaster:
	$(call go-build,github.com/gogo/protobuf/protoc-gen-gogofaster)

CLEAN += $(TGTS_$(d))
include mk/footer.mk
