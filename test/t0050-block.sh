#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test block command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_DIR="$(pwd)/.go-ipfs" &&
	ipfs init
'

test_expect_success "'ipfs block put' succeeds" '
	echo "Hello Mars!" >expected_in &&
	ipfs block put <expected_in >actual_out
'

test_expect_success "'ipfs block put' output looks good" '
	HASH="QmRKqGMAM6EZngbpjSqrvYzq5Qd8b1bSWymjSUY9zQSNDk" &&
	echo "added as '\''$HASH'\''" >expected_out &&
	test_cmp expected_out actual_out
'

test_expect_success "'ipfs block get' succeeds" '
	ipfs block get $HASH >actual_in
'

test_expect_success "'ipfs block get' output looks good" '
	test_cmp expected_in actual_in
'

test_done
