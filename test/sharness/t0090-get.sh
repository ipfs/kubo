#!/bin/sh
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test get command"

. lib/test-lib.sh

test_init_ipfs

test_ipfs_get_flag() {
    ext="$1"; shift
    tar_flag="$1"; shift
    flag="$@"

    test_expect_success "ipfs get $flag succeeds" '
        ipfs get "$HASH" '"$flag"' >actual
    '

    test_expect_success "ipfs get $flag output looks good" '
		printf "%s\n\n" "Saving archive to $HASH$ext" >expected &&
		test_cmp expected actual
	'

    test_expect_success "ipfs get $flag archive output is valid" '
		tar "$tar_flag" "$HASH$ext" &&
		test_cmp "$HASH" data &&
		rm "$HASH$ext" &&
		rm "$HASH"
	'
}

# we use a function so that we can run it both offline + online
test_get_cmd() {

	test_expect_success "'ipfs get --help' succeeds" '
		ipfs get --help >actual
	'

	test_expect_success "'ipfs get --help' output looks good" '
		egrep "ipfs get.*<ipfs-path>" actual >/dev/null ||
		test_fsh cat actual
	'

	test_expect_success "ipfs get succeeds" '
		echo "Hello Worlds!" >data &&
		HASH=`ipfs add -q data` &&
		ipfs get "$HASH" >actual
	'

	test_expect_success "ipfs get output looks good" '
		printf "%s\n\n" "Saving file(s) to $HASH" >expected &&
		test_cmp expected actual
	'

	test_expect_success "ipfs get file output looks good" '
		test_cmp "$HASH" data
	'

	test_expect_success "ipfs get DOES NOT error when trying to overwrite a file" '
		ipfs get "$HASH" >actual &&
		rm "$HASH"
	'

    test_ipfs_get_flag ".tar" "-xf" -a

    test_ipfs_get_flag ".tar.gz" "-zxf" -a -C

    test_ipfs_get_flag ".tar.gz" "-zxf" -a -C -l 9

	test_expect_success "ipfs get succeeds (directory)" '
		mkdir -p dir &&
		touch dir/a &&
		mkdir -p dir/b &&
		echo "Hello, Worlds!" >dir/b/c &&
		HASH2=`ipfs add -r -q dir | tail -n 1` &&
		ipfs get "$HASH2" >actual
	'

	test_expect_success "ipfs get output looks good (directory)" '
		printf "%s\n\n" "Saving file(s) to $HASH2" >expected &&
		test_cmp expected actual
	'

	test_expect_success "ipfs get output is valid (directory)" '
		test_cmp dir/a "$HASH2"/a &&
		test_cmp dir/b/c "$HASH2"/b/c &&
		rm -r "$HASH2"
	'

	test_expect_success "ipfs get -a -C succeeds (directory)" '
		ipfs get "$HASH2" -a -C >actual
	'

	test_expect_success "ipfs get -a -C output looks good (directory)" '
		printf "%s\n\n" "Saving archive to $HASH2.tar.gz" >expected &&
		test_cmp expected actual
	'

	test_expect_success "gzipped tar archive output is valid (directory)" '
		tar -zxf "$HASH2".tar.gz &&
		test_cmp dir/a "$HASH2"/a &&
		test_cmp dir/b/c "$HASH2"/b/c &&
		rm -r "$HASH2"
	'

	test_expect_success "ipfs get ../.. should fail" '
		echo "Error: invalid ipfs ref path" >expected &&
		test_must_fail ipfs get ../.. 2>actual &&
		test_cmp expected actual
	'
}

# should work offline
test_get_cmd

# should work online
test_launch_ipfs_daemon
test_get_cmd
test_kill_ipfs_daemon

test_done
