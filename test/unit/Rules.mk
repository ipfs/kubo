include mk/header.mk

CLEAN += $(d)/gotest.json $(d)/gotest.junit.xml

$(d)/gotest.junit.xml: cmd/ipfs/ipfs test/bin/gotestsum coverage/unit_tests.coverprofile
	gotestsum --no-color --junitfile $@ --raw-command cat $(@D)/gotest.json

include mk/footer.mk
