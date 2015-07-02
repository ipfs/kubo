#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs swarm command"

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success 'disconnected: peers is empty' '
	ipfs swarm peers >actual &&
	test_must_be_empty actual
'

test_expect_success 'disconnected: addrs local has localhost' '
	ipfs swarm addrs local >actual &&
	grep "/ip4/127.0.0.1" actual
'

test_expect_success 'disconnected: addrs local matches ipfs id' '
	ipfs id -f="<addrs>\\n" | sort >expected &&
	ipfs swarm addrs local --id | sort >actual &&
	test_cmp expected actual
'

test_kill_ipfs_daemon

test_done
