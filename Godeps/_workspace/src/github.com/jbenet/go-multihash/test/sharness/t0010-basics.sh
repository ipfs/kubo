#!/bin/sh
#
# Copyright (c) 2015 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Basic tests"

. lib/test-lib.sh

test_expect_success "current dir is writable" '
	echo "It works!" >test.txt
'

test_expect_success "multihash is available" '
	type multihash
'

test_expect_success "multihash help output looks good" '
	test_must_fail multihash -h 2>help.txt &&
	cat help.txt | egrep -i "^usage:" >/dev/null &&
	cat help.txt | egrep -i "multihash .*options.*file" >/dev/null
'

test_done
