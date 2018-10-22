gx-path = gx/ipfs/$(shell gx deps find $(1))/$(1)

# Rebuild the lockfile iff it exists.
gx-deps: $(wildcard gx-lock.json)
	if test -e gx-lock.json; then gx-go rw --undo && gx lock-install; else rm -rf vendor && gx install --global && gx-go rw; fi

.PHONY: gx-deps

lock: gx-lock.json
.PHONY: lock

gx-lock.json: package.json
	gx-go lock-gen >gx-lock.json

ifneq ($(IPFS_GX_USE_GLOBAL),1)
gx-deps: bin/gx bin/gx-go
endif
.PHONY: gx-deps

test_gx_imports:
	bin/test-gx-imports

ifeq ($(tarball-is),0)
DEPS_GO += gx-deps
endif
