#!/bin/sh
#
# Copyright (c) 2015 Cayman Nava
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test Content-Type for channel-streaming commands"

. lib/test-lib.sh

test_init_ipfs

test_ls_cmd() {

	test_expect_success "cat HTTP API call succeeds" '
		echo "hello test" >test.txt &&
		ipfs add test.txt &&
        curl -i http://localhost:$PORT_API/api/v0/cat?arg=QmRmPLc1FsPAn8F8F9DQDEYADNX5ER2sgqiokEvqnYknVW  >cat_output
	'

	test_expect_success "Search for Transfer-Encoding succeeds" '
		cat cat_output | grep -c Transfer-Encoding >has_te
	'

	test_expect_success "Search for Content-Length succeeds" '
		cat cat_output | grep -cw ^Content-Length >has_cl ||
		echo succeed
	'

	test_expect_success "Only one of Transfer-Encoding or Content-Length exists" '
		! [ `cat has_te` -ne 0 -a `cat has_cl` -ne 0 ]
	'
}

# should work online (only)
test_launch_ipfs_daemon
test_ls_cmd
test_kill_ipfs_daemon

test_done
