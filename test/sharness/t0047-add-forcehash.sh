#!/usr/bin/env bash

test_description="Test add force hash commands"

. lib/test-lib.sh


test_add_force_hash() {

    test_expect_success "'ipfs add succeeds" '
    echo "Hello Worlds!" >mountdir/hello.txt &&
    ipfs add mountdir/hello.txt >actual
    '

     test_expect_success "ipfs add output looks good" '
    HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
    echo "added $HASH hello.txt" >expected &&
    test_cmp expected actual
  '
    test_expect_success "'ipfs add --hash=sha2-256 succeeds" '
    ipfs add --hash=sha2-256 mountdir/hello.txt >actual
    '
    test_expect_success "'ipfs add --hash=sha1 --allow-insecure-hash-function succeeds" '
    ipfs add --hash=sha1 --allow-insecure-hash-function mountdir/hello.txt >actual
    '
    test_expect_success "'ipfs add --hash=sha2-256 --allow-insecure-hash-function succeeds" '
    ipfs add --hash=sha2-256 --allow-insecure-hash-function mountdir/hello.txt >actual
    '
    test_expect_success "'ipfs add --allow-insecure-hash-function succeeds" '
    ipfs add --hash=sha2-256 --allow-insecure-hash-function mountdir/hello.txt >actual
    '
    test_expect_success "ipfs add --allow-insecure-hash-function succeeds output looks good" '
    HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
    echo "added $HASH hello.txt" >expected &&
    test_cmp expected actual
    '
    test_expect_failure "ipfs -add --hash=sha1 fails" '
    ipfs add --hash=sha1 mountdir/hello.txt >actual
    '

    test_expect_failure 'ipfs -add --hash=sha1 correct out' '
    test_cmp expected actual
    '

}

test_init_ipfs
test_add_force_hash

test_launch_ipfs_daemon
test_add_force_hash
test_kill_ipfs_daemon

test_done