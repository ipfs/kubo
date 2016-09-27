#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test client mode dht"

. lib/test-lib.sh

check_file_fetch() {
	node=$1
	fhash=$2
	fname=$3

	test_expect_success "can fetch file" '
		ipfsi $node cat $fhash > fetch_out
	'

	test_expect_success "file looks good" '
		test_cmp $fname fetch_out
	'
}

check_dir_fetch() {
	node=$1
	ref=$2

	test_expect_success "node can fetch all refs for dir" '
		ipfsi $node refs -r $ref > /dev/null
	'
}

run_single_file_test() {
	test_expect_success "add a file on node1" '
		random 1000000 > filea &&
		FILEA_HASH=$(ipfsi 1 add -q filea)
	'

	check_file_fetch 4 $FILEA_HASH filea
	check_file_fetch 3 $FILEA_HASH filea
	check_file_fetch 2 $FILEA_HASH filea
	check_file_fetch 1 $FILEA_HASH filea
	check_file_fetch 0 $FILEA_HASH filea
}

run_random_dir_test() {
	test_expect_success "create a bunch of random files" '
		random-files -depth=4 -dirs=5 -files=8 foobar > /dev/null
	'

	test_expect_success "add those on node 2" '
		DIR_HASH=$(ipfsi 2 add -r -q foobar | tail -n1)
	'

	check_dir_fetch 0 $DIR_HASH
	check_dir_fetch 1 $DIR_HASH
	check_dir_fetch 2 $DIR_HASH
	check_dir_fetch 3 $DIR_HASH
	check_dir_fetch 4 $DIR_HASH
}

run_advanced_test() {

	run_single_file_test

	run_random_dir_test

	test_expect_success "shut down nodes" '
		iptb stop
	'
}

test_expect_success "set up testbed" '
	iptb init -n 10 -p 0 -f --bootstrap=none
'

test_expect_success "start up nodes" '
	iptb start [0-7] &&
	iptb start [8-9] --args="--routing=dhtclient"
'

test_expect_success "connect up nodes" '
	iptb connect [1-9] 0
'

test_expect_success "add a file on a node in client mode" '
	random 1000000 > filea &&
	FILE_HASH=$(ipfsi 8 add -q filea)
'

test_expect_success "retrieve that file on a client mode node" '
	check_file_fetch 9 $FILE_HASH filea
'

run_advanced_test

test_done
