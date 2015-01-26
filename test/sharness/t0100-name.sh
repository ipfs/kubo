#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "'ipfs name publish' succeeds" '
	PEERID=`ipfs id -format="<id>"` &&
	HASH=QmYpv2VEsxzTTXRYX3PjDg961cnJE3kY1YDXLycHGQ3zZB &&
	ipfs name publish $HASH > publish_out &&
	echo Published name $PEERID to $HASH > expected1 &&
	test_cmp publish_out expected1

'

test_expect_success "'ipfs name resolve' succeeds" '
	ipfs name resolve $PEERID > output &&
	printf "%s" $HASH > expected2 &&
	test_cmp output expected2
'

test_done
