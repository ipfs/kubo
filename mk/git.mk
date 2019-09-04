# First try to "describe" the state. This tells us if the state is dirty.
# If that fails (e.g., we're building a docker image and have an empty objects
# directory), assume the source isn't dirty and build anyways.
git-hash:=$(shell git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)
