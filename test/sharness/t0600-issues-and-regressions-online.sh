#!/bin/sh

test_description="Tests for various fixed issues and regressions."

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

# Tests go here

test_expect_success "commands command with flag flags works via HTTP API - #2301" '
	curl "http://$API_ADDR/api/v0/commands?flags" | grep "verbose"
'

test_expect_success "ipfs refs local over HTTP API returns NDJOSN not flat - #2803" '
	echo "Hello World" | ipfs add &&
	curl "http://$API_ADDR/api/v0/refs/local" | grep "Ref" | grep "Err"
'

test_expect_success "args expecting stdin dont crash when not given" '
	curl "$API_ADDR/api/v0/bootstrap/add" > result
'

test_expect_success "no panic traces on daemon" '
	test_expect_failure grep "nil pointer dereference" daemon_err
'

test_kill_ipfs_daemon

test_done

