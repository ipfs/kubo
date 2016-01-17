#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

# changing the bootstrap peers will require changing it in two places :)
test_description="test node bootstrapping"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "disable mdns" '
	ipfs config Discovery.MDNS.Enabled false --json
'

test_launch_ipfs_daemon

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

test_expect_success "setup iptb nodes" '
	iptb init -n 5 -f --bootstrap=none --port=0
'

test_expect_success "set bootstrap addrs" '
	bsn_peer_id=$(ipfs id -f "<id>") &&
	BADDR="/ip4/127.0.0.1/tcp/$PORT_SWARM/ipfs/$bsn_peer_id" &&
	ipfsi 0 bootstrap add $BADDR &&
	ipfsi 1 bootstrap add $BADDR &&
	ipfsi 2 bootstrap add $BADDR &&
	ipfsi 3 bootstrap add $BADDR &&
	ipfsi 4 bootstrap add $BADDR
'

test_expect_success "start up iptb nodes" '
	iptb start --wait
'

test_expect_success "check peers works" '
	ipfs swarm peers > peers_out
'

test_expect_success "correct number of peers" '
	test `cat peers_out | wc -l` == 5
'

test_kill_ipfs_daemon

test_expect_success "bring down iptb nodes" '
	iptb stop
'

test_done
