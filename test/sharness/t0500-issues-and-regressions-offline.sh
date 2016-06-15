#!/bin/sh

test_description="Tests for various fixed issues and regressions."

. lib/test-lib.sh

test_init_ipfs

# Tests go here

test_expect_success "ipfs init with occupied input works - #2748" '
	export IPFS_PATH="ipfs_path"
	echo "" | time-out ipfs init &&
	rm -rf ipfs_path
'

test_done
