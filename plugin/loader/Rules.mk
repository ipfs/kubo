include mk/header.mk

IPFS_PLUGINS ?= 
export IPFS_PLUGINS

$(d)/preload.go: d:=$(d)
$(d)/preload.go: $(d)/preload_list $(d)/preload.sh ALWAYS
	$(d)/preload.sh > $@
	$(GOCC) fmt $(go-global-flags) $@ >/dev/null

DEPS_GO += $(d)/preload.go

include mk/footer.mk
