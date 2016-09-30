#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "create some files" '
	echo A > fileA &&
	echo B > fileB &&
	echo C > fileC
'

test_expect_success "add files all at once" '
	ipfs add -q fileA fileB fileC > hashes
'

test_expect_failure "unpin one of the files" '
	ipfs pin rm `head -1 hashes` > pin-out
'

test_expect_failure "unpin output looks good" '
	echo "unpinned `head -1 hashes`" > pin-expect
	test_cmp pin-expect pin-out
'


test_done
