#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_launch_ipfs_daemon_and_mount

test_expect_success "ipfs add succeeds" '
	echo "Hello Worlds!" >mountdir/hello.txt &&
	ipfs add mountdir/hello.txt >actual
'

test_expect_success "ipfs add output looks good" '
	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
	echo "added $HASH mountdir/hello.txt" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat succeeds" '
	ipfs cat $HASH >actual
'

test_expect_success "ipfs cat output looks good" '
	echo "Hello Worlds!" >expected &&
	test_cmp expected actual
'

test_expect_success FUSE "cat ipfs/stuff succeeds" '
	cat ipfs/$HASH >actual
'

test_expect_success FUSE "cat ipfs/stuff looks good" '
	test_cmp expected actual
'

test_expect_success "go-random is installed" '
	type random
'

test_expect_success EXPENSIVE "generate 100MB file using go-random" '
	random 104857600 42 >mountdir/bigfile
'

test_expect_success EXPENSIVE "sha1 of the file looks ok" '
	echo "885b197b01e0f7ff584458dc236cb9477d2e736d  mountdir/bigfile" >sha1_expected &&
	shasum mountdir/bigfile >sha1_actual &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success EXPENSIVE "ipfs add bigfile succeeds" '
	ipfs add mountdir/bigfile >actual
'

test_expect_success EXPENSIVE "ipfs add bigfile output looks good" '
	HASH="QmWXysX1oysyjTqd5xGM2T1maBaVXnk5svQv4GKo5PsGPo" &&
	echo "added $HASH mountdir/bigfile" >expected &&
	test_cmp expected actual
'

test_expect_success EXPENSIVE "ipfs cat succeeds" '
	ipfs cat $HASH | shasum >sha1_actual
'

test_expect_success EXPENSIVE "ipfs cat output looks good" '
	ipfs cat $HASH >actual &&
	test_cmp mountdir/bigfile actual
'

test_expect_success EXPENSIVE "ipfs cat output shasum looks good" '
	echo "885b197b01e0f7ff584458dc236cb9477d2e736d  -" >sha1_expected &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile succeeds" '
	cat ipfs/$HASH | shasum >sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile looks good" '
	test_cmp sha1_expected sha1_actual
'

test_kill_ipfs_daemon

test_done
