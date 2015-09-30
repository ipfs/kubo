#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

export IPTB_ROOT="`pwd`/.iptb"
export DEBUG=true

ipfsi() {
	local dir=$1; shift; IPFS_PATH="$IPTB_ROOT/$dir" ipfs $@
}

setup_iptb() {
	test_expect_success "iptb init" '
		iptb init -n4 --bootstrap none --port 0
	'

	test_expect_success "set configs up" '
		for i in `seq 0 3`
		do
			ipfsi $i config Ipns.RepublishPeriod 20s
		done
	'

	test_expect_success "start up nodes" '
		iptb start
	'

	test_expect_success "connect nodes" '
		iptb connect 0 1 &&
		iptb connect 0 2 &&
		iptb connect 0 3
	'

	test_expect_success "nodes have connections" '
		ipfsi 0 swarm peers | grep ipfs &&
		ipfsi 1 swarm peers | grep ipfs &&
		ipfsi 2 swarm peers | grep ipfs &&
		ipfsi 3 swarm peers | grep ipfs
	'
}

teardown_iptb() {
	test_expect_success "shut down nodes" '
		iptb kill
	'
}

verify_can_resolve() {
	node=$1
	name=$2
	expected=$3

	test_expect_success "node can resolve entry" '
		ipfsi $node name resolve $name > resolve
	'

	test_expect_success "output looks right" '
		printf /ipfs/$expected > expected &&
		test_cmp resolve expected
	'
}

verify_cannot_resolve() {
	node=$1
	name=$2

	echo "verifying resolution fails on node $node"
	test_expect_success "node cannot resolve entry" '
		# TODO: this should work without the timeout option
		# but it currently hangs for some reason every so often
		test_expect_code 1 ipfsi $node name resolve --timeout=300ms $name
	'
}

setup_iptb

test_expect_success "publish succeeds" '
	HASH=$(echo "foobar" | ipfsi 1 add -q) &&
	ipfsi 1 name publish -t 5s $HASH
'

test_expect_success "other nodes can resolve" '
	id=$(ipfsi 1 id -f "<id>") &&
	verify_can_resolve 0 $id $HASH &&
	verify_can_resolve 1 $id $HASH &&
	verify_can_resolve 2 $id $HASH &&
	verify_can_resolve 3 $id $HASH
'

test_expect_success "after five seconds, records are invalid" '
	go-sleep 5s &&
	verify_cannot_resolve 0 $id &&
	verify_cannot_resolve 1 $id &&
	verify_cannot_resolve 2 $id &&
	verify_cannot_resolve 3 $id
'

test_expect_success "republisher fires after twenty seconds" '
	go-sleep 15s &&
	verify_can_resolve 0 $id $HASH &&
	verify_can_resolve 1 $id $HASH &&
	verify_can_resolve 2 $id $HASH &&
	verify_can_resolve 3 $id $HASH
'

teardown_iptb

test_done
