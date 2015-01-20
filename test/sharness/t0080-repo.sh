#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "'ipfs add afile' succeeds" '
	echo "some text" > afile
	HASH=`ipfs add -q afile`
	echo -n $HASH > hashfile
'

test_expect_success "added file was pinned" '
	ipfs pin ls -type=recursive | grep `cat hashfile`
'

test_expect_success "'ipfs repo gc' doesnt remove file" '
	ipfs repo gc
	ipfs cat `cat hashfile` > out
	test_cmp out afile
'

test_expect_success "'ipfs pin rm' succeeds" '
	echo unpinned `cat hashfile` > expected1
	ipfs pin rm -r `cat hashfile` > actual1
	test_cmp expected1 actual1
'

test_expect_success "file no longer pinned" '
	echo -n "" > expected2
	ipfs pin ls -type=recursive > actual2
	test_cmp expected2 actual2
'

test_expect_success "recursively pin afile" '
	ipfs pin add -r `cat hashfile`
'

test_expect_success "pinning directly should fail now" '
	echo Error: pin: `cat hashfile` already pinned recursively > expected3
	ipfs pin add `cat hashfile` 2> actual3
	test_cmp expected3 actual3
'

test_expect_success "'ipfs pin rm <hash>' should fail" '
	echo Error: `cat hashfile` is pinned recursively > expected4
	ipfs pin rm `cat hashfile` 2> actual4
	test_cmp expected4 actual4
'

test_expect_success "remove recursive pin, add direct" '
	echo unpinned `cat hashfile` > expected5
	ipfs pin rm -r `cat hashfile` > actual5
	test_cmp expected5 actual5
	ipfs pin add `cat hashfile`
'

test_expect_success "remove direct pin" '
	echo unpinned `cat hashfile` > expected6
	ipfs pin rm `cat hashfile` > actual6
	test_cmp expected6 actual6
'

test_expect_success "'ipfs repo gc' removes file" '
	echo removed `cat hashfile` > expected7
	ipfs repo gc > actual7
	test_cmp expected7 actual7
'

test_expect_success "'ipfs refs local' no longer shows file" '
	echo -n "" > expected8
	ipfs refs local > actual8
	test_cmp expected8 actual8
'


test_kill_ipfs_daemon

test_done
