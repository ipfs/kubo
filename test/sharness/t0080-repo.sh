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

# TODO: run gc, then ipfs cat file, should still be there

test_expect_success "'ipfs pin rm' succeeds" '
	echo Unpinned `cat hashfile` > expected
	ipfs pin rm `cat hashfile` > actual
	test_cmp expected actual
'

test_expect_success "file no longer pinned" '
	echo -n "" > expected
	ipfs pin ls -type=recursive > actual
	test_cmp expected actual
'

test_expect_success "recursively pin afile" '
	ipfs pin add -r `cat hashfile`
'

test_expect_success "pinning directly should fail now" '
	echo "Error: pin: Key already pinned recursively." > expected
	ipfs pin add `cat hashfile` 2> actual
	test_cmp expected actual
'

test_expect_success "remove recursive pin, add direct" '
	echo Unpinned `cat hashfile` > expected
	ipfs pin rm `cat hashfile` > actual
	test_cmp expected actual
	ipfs pin add `cat hashfile`
'

test_expect_success "remove direct pin" '
	ipfs pin rm `cat hashfile` > actual
	test_cmp expected actual
'


test_kill_ipfs_daemon

test_done
