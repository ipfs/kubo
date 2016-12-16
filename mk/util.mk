# util functions
ifeq ($(OS),Windows_NT)
	WINDOWS :=1
	?exe :=.exe # windows compat
else
	?exe :=
endif

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
