#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test multiple ipfs nodes"

. lib/test-lib.sh

export IPTB_ROOT="`pwd`/.iptb"

test_expect_success "set up a few nodes" '
	iptb -n=3 init &&
	iptb -wait start
'

test_expect_success "add a file on node1" '
	export IPFS_PATH="$IPTB_ROOT/1" &&
	random 1000000 > filea &&
	FILEA_HASH=$(ipfs add -q filea)
'

test_expect_success "cat that file on node2" '
	export IPFS_PATH="$IPTB_ROOT/2" &&
	ipfs cat $FILEA_HASH >fileb
'

test_expect_success "verify files match" '
	multihash filea > expected1 &&
	multihash fileb > actual1 &&
	test_cmp actual1 expected1
'

test_expect_success "shut down nodes" '
	iptb stop
'

test_done
