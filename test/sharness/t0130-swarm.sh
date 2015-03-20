#!/bin/bash

test_description="Test swarm command"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "'ipfs swarm addrs' succeeds" '
	ipfs swarm addrs >swarm_addrs_out 2>swarm_addrs_err ||
	test_fsh cat swarm_addrs_out || test_fsh cat swarm_addrs_err
'

test_swarm_addrs_output() {
	cat "$1" |
	while IFS='' read -r line || [[ -n $line ]]
	do
		grep '^Qm[0-9A-Za-z]\{44\} ([1-9][0-9]*)$' <<<"$line" >/dev/null || return
		addr_count="$(cat <<<"$line" | sed "s/^.*(//" | sed "s/)$//")"
		for i in $(seq "$addr_count")
		do
			IFS='' read -r line || [[ -n $line ]] || return
			grep $'^\t/[^/]*/[^/]*/[^/]*/[^/]*$' <<<"$line" >/dev/null || return
		done
	done
}

test_expect_success "'ipfs swarm addrs' output looks good" '
	test_swarm_addrs_output swarm_addrs_out ||
	test_fsh echo $? || test_fsh cat swarm_addrs_out || test_fsh cat swarm_addrs_err
'

test_kill_ipfs_daemon

test_done
