#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs swarm command"

AF1="/ip4/192.168.0.0/ipcidr/16"
AF2="/ip4/127.0.0.0/ipcidr/8"
AF3="/ip6/2008:bcd::/ipcidr/32"
AF4="/ip4/172.16.0.0/ipcidr/12"

. lib/test-lib.sh

test_init_ipfs

test_swarm_filter_cmd() {
	printf "" > list_expected
	for AF in "$@"
	do
		echo "$AF" >>list_expected
	done

	test_expect_success "'ipfs swarm filters' succeeds" '
		ipfs swarm filters > list_actual
	'

	test_expect_success "'ipfs swarm filters' output looks good" '
		test_sort_cmp list_actual list_expected
	'
}

test_swarm_filters() {

	# expect first address from config
	test_swarm_filter_cmd $AF1 $AF4

	ipfs swarm filters rm all

	test_swarm_filter_cmd

	test_expect_success "'ipfs swarm filter add' succeeds" '
		ipfs swarm filters add $AF1 $AF2 $AF3
	'

	test_swarm_filter_cmd $AF1 $AF2 $AF3

	test_expect_success "'ipfs swarm filter rm' succeeds" '
		ipfs swarm filters rm $AF2 $AF3
	'

	test_swarm_filter_cmd $AF1

	test_expect_success "'ipfs swarm filter add' succeeds" '
		ipfs swarm filters add $AF4 $AF2
	'

	test_swarm_filter_cmd $AF1 $AF2 $AF4

	test_expect_success "'ipfs swarm filter rm' succeeds" '
		ipfs swarm filters rm $AF1 $AF2 $AF4
	'

	test_swarm_filter_cmd
}

test_expect_success "init without any filters" '
	echo "null" >expected &&
	ipfs config Swarm.AddrFilters >actual &&
	test_cmp expected actual
'

test_expect_success "adding addresses to the config to filter succeeds" '
	ipfs config --json Swarm.AddrFilters "[\"$AF1\", \"$AF4\"]"
'

test_launch_ipfs_daemon

test_swarm_filters

test_kill_ipfs_daemon

test_done
