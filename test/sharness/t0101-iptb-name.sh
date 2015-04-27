#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

export IPTB_ROOT="`pwd`/.iptb"

test_expect_success "set up an iptb cluster" '
	iptb -n=4 init &&
	iptb -wait start
'

test_expect_success "add an obect on one node" '
	export IPFS_PATH="$IPTB_ROOT/1" &&
	echo "ipns is super fun" > file &&
	HASH_FILE=`ipfs add -q file`
'

test_expect_success "publish that object as an ipns entry" '
	ipfs name publish $HASH_FILE
'

test_expect_success "add an entry on another node pointing to that one" '
	export IPFS_PATH="$IPTB_ROOT/2" &&
	NODE1_ID=`iptb get id 1` &&
	ipfs name publish /ipns/$NODE1_ID
'

test_expect_success "cat that entry on a third node" '
	export IPFS_PATH="$IPTB_ROOT/3" &&
	NODE2_ID=`iptb get id 2` &&
	ipfs cat /ipns/$NODE2_ID > output
'

test_expect_success "ensure output was the same" '
	test_cmp file output
'

test_expect_success "shut down iptb" '
	iptb stop
'

test_done
