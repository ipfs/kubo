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

# Normalize `origin` to `host/org/repo` for runtime fork detection via
# Version.AgentSuffix. Handles ssh and https forms, strips `.git`, drops
# userinfo. Empty when no git, no `origin`, or git is unavailable.
git-origin:=$(shell git remote get-url origin 2>/dev/null \
  | sed -E -e 's|^git@([^:]+):|\1/|' \
           -e 's|^[a-z]+://||' \
           -e 's|^[^/]+@||' \
           -e 's|\.git$$||')
