include mk/header.mk

$(d)_plugins:=$(d)/git
$(d)_plugins_so:=$(addsuffix .so,$($(d)_plugins))

$($(d)_plugins_so): $$(DEPS_GO) ALWAYS
	go build -buildmode=plugin -i $(go-flags-with-tags) -o "$@" "$(call go-pkg-name,$(basename $@))"

CLEAN += $($(d)_plugins_so)

build_plugins: $($(d)_plugins_so)


include mk/footer.mk
