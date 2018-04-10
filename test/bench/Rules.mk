include mk/header.mk

DEPS_$(d) := test/bin/gobench-to-json

$(d)/gobench.out:
	go test -bench . -run '^Benchmark' ./... -benchmem | tee $(@D)/gobench.out
.PHONY: $(d)/gobench.out

$(d)/bench-results.json: $$(DEPS_$(d)) $(d)/gobench.out
	cat $(@D)/gobench.out | test/bin/gobench-to-json $(@D)/bench-results.json

include mk/footer.mk
