# util functions
OS ?= $(shell sh -c 'uname -s 2>/dev/null || echo not')
ifeq ($(OS),Windows_NT)
	WINDOWS :=1
	?exe :=.exe # windows compat
	PATH_SEP :=;
else
	?exe :=
	PATH_SEP :=:
endif

# Platforms are now defined in .github/build-platforms.yml
# The cmd/ipfs-try-build target is deprecated in favor of GitHub Actions
# Use 'make supported' to see the list of platforms

space:=$() $()
comma:=,
join-with=$(subst $(space),$1,$(strip $2))

# debug target, prints variable. Example: `make print-GOFLAGS`
print-%:
	@echo $*=$($*)

# phony target that will mean that recipe is always executed
ALWAYS:
.PHONY: ALWAYS
