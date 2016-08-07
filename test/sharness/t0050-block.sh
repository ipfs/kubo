#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test block command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "'ipfs block put' succeeds" '
	echo "Hello Mars!" >expected_in &&
	ipfs block put <expected_in >actual_out
'

test_expect_success "'ipfs block put' output looks good" '
	HASH="QmRKqGMAM6EZngbpjSqrvYzq5Qd8b1bSWymjSUY9zQSNDk" &&
	echo "$HASH" >expected_out &&
	test_cmp expected_out actual_out
'

test_expect_success "'ipfs block get' succeeds" '
	ipfs block get $HASH >actual_in
'

test_expect_success "'ipfs block get' output looks good" '
	test_cmp expected_in actual_in
'

test_expect_success "'ipfs block stat' succeeds" '
  ipfs block stat $HASH >actual_stat
'

test_expect_success "'ipfs block get' output looks good" '
  echo "Key: $HASH" >expected_stat &&
  echo "Size: 12" >>expected_stat &&
  test_cmp expected_stat actual_stat
'

test_expect_success "'ipfs block stat' with nothing from stdin doesnt crash" '
	test_expect_code 1 ipfs block stat < /dev/null 2> stat_out
'

test_expect_success "no panic in output" '
	test_expect_code 1 grep "panic" stat_out
'

test_done
