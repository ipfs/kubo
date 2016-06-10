#!/bin/sh

test_description="Tests for various fixed issues and regressions."

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

# Tests go here

test_expect_sucess "commands command with flag flags works via HTTP API - #2301" '
	curl "http://$API_ADDR/api/v0/commands?flags" | grep "verbose"
'

test_expect_sucess "ipfs refs local over HTTP API returns NDJOSN not flat - #2803" '
	echo "Hello World" | ipfs add &&
	curl "http://$API_ADDR/api/v0/refs/local" | grep "Ref" | grep "Err"
'

test_kill_ipfs_daemon

test_done

