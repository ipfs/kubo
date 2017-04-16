#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test global enable sharding flag"

. lib/test-lib.sh

test_expect_success "set up test data" '
	mkdir testdata
	for i in `seq 2000`
	do
		echo $i > testdata/file$i
	done
'

test_add_large_dir() {
	exphash="$1"
	test_expect_success "ipfs add on very large directory succeeds" '
		ipfs add -r -q testdata | tail -n1 > sharddir_out &&
		echo "$exphash" > sharddir_exp &&
		test_cmp sharddir_exp sharddir_out
	'
}

test_init_ipfs

UNSHARDED="QmavrTrQG4VhoJmantURAYuw3bowq3E2WcvP36NRQDAC1N"
test_add_large_dir "$UNSHARDED"

test_launch_ipfs_daemon

test_add_large_dir "$UNSHARDED"

test_kill_ipfs_daemon

test_expect_success "enable sharding" '
	ipfs config --json Experimental.ShardingEnabled true
'

SHARDED="QmSCJD1KYLhVVHqBK3YyXuoEqHt7vggyJhzoFYbT8v1XYL"
test_add_large_dir "$SHARDED"

test_launch_ipfs_daemon

test_add_large_dir "$SHARDED"

test_kill_ipfs_daemon

test_expect_success "sharded and unsharded output look the same" '
	ipfs ls "$SHARDED" | sort > sharded_out &&
	ipfs ls "$UNSHARDED" | sort > unsharded_out &&
	test_cmp sharded_out unsharded_out
'

test_done
