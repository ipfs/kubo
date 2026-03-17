include mk/header.mk

CLEAN += $(d)/gotest.json $(d)/gotest.junit.xml

# Convert gotest.json (produced by test_unit) to JUnit XML format
$(d)/gotest.junit.xml: test/bin/gotestsum $(d)/gotest.json
	gotestsum --no-color --junitfile $@ --raw-command cat $(@D)/gotest.json

include mk/footer.mk
