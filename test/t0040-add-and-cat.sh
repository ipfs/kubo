#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

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

test_expect_success "ipfs add succeeds" '
	echo "Hello Worlds!" >mountdir/hello.txt &&
	ipfs add mountdir/hello.txt >actual
'

test_expect_success "ipfs add output looks good" '
	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
	echo "added $HASH $(pwd)/mountdir/hello.txt" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat succeeds" '
	ipfs cat $HASH >actual
'

test_expect_success "ipfs cat output looks good" '
	echo "Hello Worlds!" >expected &&
	test_cmp expected actual
'

test_expect_success "cat ipfs/stuff succeeds" '
	cat ipfs/$HASH >actual
'

test_expect_success "cat ipfs/stuff looks good" '
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
