# util functions
ifeq ($(OS),Windows_NT)
	WINDOWS :=1
	?exe :=.exe # windows compat
	PATH_SEP :=;
else
	?exe :=
	PATH_SEP :=:
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
