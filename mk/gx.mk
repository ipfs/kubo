gx-path = gx/ipfs/$(shell gx deps find $(1))/$(1)

gx-deps: bin/gx bin/gx-go $(CHECK_GO)
	gx install --global >/dev/null 2>&1

DEPS_GO += gx-deps
