include mk/header.mk

CLEAN += $(d)/gotest.json $(d)/gotest.junit.xml

$(d)/gotest.junit.xml: clean test/bin/json-to-junit coverage/unit_tests.coverprofile
	cat $(@D)/gotest.json | json-to-junit > $(@D)/gotest.junit.xml

include mk/footer.mk
