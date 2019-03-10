include mk/header.mk

dist_root_$(d)="/ipfs/QmPrXH9jRVwvd7r5MC5e6nV4uauQGzLk1i2647Ye9Vbbwe"

TGTS_$(d) := $(d)/gx $(d)/gx-go
DISTCLEAN += $(wildcard $(d)/gx-v*) $(wildcard $(d)/gx-go-v*) $(d)/tmp

PATH := $(realpath $(d)):$(PATH)

$(TGTS_$(d)):
	rm -f $@$(?exe)
ifeq ($(WINDOWS),1)
	cp $^$(?exe) $@$(?exe)
else
	ln -s $(notdir $^) $@
endif

CLEAN += $(TGTS_$(d))
include mk/footer.mk
