# First try to "describe" the state. This tells us if the state is dirty.
# If that fails (e.g., we're building a docker image and have an empty objects
# directory), assume the source isn't dirty and build anyways.
git-hash:=$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)

# Detect if HEAD is a clean, tagged release. Used to omit redundant commit
# hash from the libp2p user agent (the version number suffices).
ifeq ($(findstring dirty,$(git-hash)),)
  git-tag:=$(shell git tag --points-at HEAD 2>/dev/null | grep '^v' | head -1)
else
  git-tag:=
endif
