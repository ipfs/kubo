#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add --no-copy"

. lib/test-lib.sh

client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}

test_add_cat_file() {
    test_expect_success "ipfs add succeeds" '
    	echo "Hello Worlds!" >mountdir/hello.txt &&
        ipfs add --no-copy mountdir/hello.txt >actual
    '

    test_expect_success "ipfs add output looks good" '
    	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
        echo "added $HASH hello.txt" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "ipfs cat succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "ipfs cat output looks good" '
    	echo "Hello Worlds!" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "fail after file move" '
        mv mountdir/hello.txt mountdir/hello2.txt
    	test_must_fail ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "okay again after moving back" '
        mv mountdir/hello2.txt mountdir/hello.txt
    	ipfs cat "$HASH" >/dev/null
    '
    
    test_expect_success "fail after file change" '
        # note: filesize shrinks
    	echo "hello world!" >mountdir/hello.txt &&
    	test_must_fail ipfs cat "$HASH" >cat.output
    '

    test_expect_success "fail after file change, same size" '
        # note: filesize does not change
    	echo "HELLO WORLDS!" >mountdir/hello.txt &&
    	test_must_fail ipfs cat "$HASH" >cat.output
    '
}

test_add_cat_5MB() {
    test_expect_success "generate 5MB file using go-random" '
    	random 5242880 41 >mountdir/bigfile
    '

    test_expect_success "sha1 of the file looks ok" '
    	echo "11145620fb92eb5a49c9986b5c6844efda37e471660e" >sha1_expected &&
    	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
    	test_cmp sha1_expected sha1_actual
    '

    test_expect_success "'ipfs add bigfile' succeeds" '
    	ipfs add --no-copy mountdir/bigfile >actual
    '

    test_expect_success "'ipfs add bigfile' output looks good" '
    	HASH="QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb" &&
    	echo "added $HASH bigfile" >expected &&
    	test_cmp expected actual
    '
    test_expect_success "'ipfs cat' succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "'ipfs cat' output looks good" '
    	test_cmp mountdir/bigfile actual
    '

    test_expect_success "fail after file move" '
        mv mountdir/bigfile mountdir/bigfile2
    	test_must_fail ipfs cat "$HASH" >/dev/null
    '
}

# should work offline

test_init_ipfs

test_add_cat_file

test_add_cat_5MB

test_done
