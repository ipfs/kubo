#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test multiple ipfs nodes"

. lib/test-lib.sh

export IPTB_ROOT="`pwd`/.iptb"

ipfsi() {
	dir="$1"
	shift
	IPFS_PATH="$IPTB_ROOT/$dir" ipfs $@
}

check_has_connection() {
	node=$1
	ipfsi $node swarm peers | grep ipfs > /dev/null
}

startup_cluster() {
	test_expect_success "start up nodes" '
		iptb start
	'

	test_expect_success "connect nodes to eachother" '
		iptb connect [1-4] 0
	'

	test_expect_success "nodes are connected" '
		check_has_connection 0 &&
		check_has_connection 1 &&
		check_has_connection 2 &&
		check_has_connection 3 &&
		check_has_connection 4
	'
}

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

test_resolve() {
	_nid="$1"
	_name="$2"
	_expected="$3"
	_comment="${4:+ ($4)}"  # Optional

	test_expect_success "IPNS resolve on node $_nid$_comment" '
		printf "%s" "$_expected" >expected &&
		ipfsi "$_nid" resolve -r "$_name" >actual &&
		test_cmp expected actual
	'
}

run_basic_test() {
	startup_cluster 

	test_expect_success "add a file on node1" '
		random 1000000 > filea &&
		FILEA_HASH=$(ipfsi 1 add -q filea)
	'

	check_file_fetch 4 $FILEA_HASH filea
	check_file_fetch 3 $FILEA_HASH filea
	check_file_fetch 2 $FILEA_HASH filea
	check_file_fetch 1 $FILEA_HASH filea
	check_file_fetch 0 $FILEA_HASH filea

	ref1="/ipfs/$HASH_WELCOME_DOCS"
	test_expect_success "IPNS publish on node 1" '
		node="$(ipfsi 1 id -f="<id>")" &&
		ipfsi 1 name publish "$ref1"
	'

	for nid in 0 1 2; do
		test_resolve "$nid" "/ipns/$node" "$ref1"
	done

	ref2="/ipfs/$FILEA_HASH"
	test_expect_success "IPNS publish again on node 1" '
		ipfsi 1 name publish "$ref2"
	'

	for nid in 0 1 2; do
		test_resolve "$nid" "/ipns/$node" "$ref1" "cached result"
	done
	for nid in 3 4; do
		test_resolve "$nid" "/ipns/$node" "$ref2" "new result"
	done

	test_expect_success "shut down nodes" '
		iptb stop
	'
}

test_expect_success "set up tcp testbed" '
	iptb init -n 5 -p 0 -f --bootstrap=none
'

run_basic_test

test_done
