#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command"

. ./test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_DIR="$(pwd)/.go-ipfs" &&
	ipfs init -b=2048
'

test_expect_success "prepare config" '
	mkdir mountdir ipfs ipns &&
	ipfs config Mounts.IPFS "$(pwd)/ipfs" &&
	ipfs config Mounts.IPNS "$(pwd)/ipns"
'

test_expect_success "ipfs mount succeeds" '
	ipfs mount mountdir >actual &
'

test_expect_success "ipfs mount output looks good" '
	IPFS_PID=$! &&
	sleep 5 &&
	echo "mounting ipfs at $(pwd)/ipfs" >expected &&
	echo "mounting ipns at $(pwd)/ipns" >>expected &&
	test_cmp expected actual
'

test_expect_success "ipfs mount is still running" '
	kill -0 $IPFS_PID
'

test_expect_success "ipfs mount can be killed" '
	kill $IPFS_PID &&
	sleep 1 &&
	! kill -0 $IPFS_PID 2>/dev/null
'

test_done
