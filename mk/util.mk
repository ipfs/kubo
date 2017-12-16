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

SUPPORTED_PLATFORMS += windows-386
SUPPORTED_PLATFORMS += windows-amd64

SUPPORTED_PLATFORMS += linux-arm
SUPPORTED_PLATFORMS += linux-arm64
SUPPORTED_PLATFORMS += linux-386
SUPPORTED_PLATFORMS += linux-amd64

SUPPORTED_PLATFORMS += darwin-386
SUPPORTED_PLATFORMS += darwin-amd64

space:=
space+=
comma:=,
join-with=$(subst $(space),$1,$(strip $2))

# debug target, prints varaible. Example: `make print-GOFLAGS`
print-%:
	@echo $*=$($*)

# phony target that will mean that recipe is always exectued
ALWAYS:
.PHONY: ALWAYS
