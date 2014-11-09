#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test installation and some basic commands"

. lib/test-lib.sh

test_expect_success "current dir is writable" '
	echo "It works!" >test.txt
'

test_expect_success "ipfs version succeeds" '
	ipfs version >version.txt
'

test_expect_success "ipfs version output looks good" '
	cat version.txt | egrep "^ipfs version [0-9]+\.[0-9]+\.[0-9]"
'

test_expect_success "ipfs help succeeds" '
	ipfs help >help.txt
'

test_expect_success "ipfs help output looks good" '
	cat help.txt | egrep "^Usage: +ipfs"
'

test_done

