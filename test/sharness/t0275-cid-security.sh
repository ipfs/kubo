#!/bin/sh
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Cid Security"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "adding using unsafe function fails with error" '
  echo foo | test_must_fail ipfs add --hash murmur3 2>add_out
'

test_expect_success "error reason is pointed out" '
  grep "insecure hash functions not allowed" add_out
'

test_expect_success "adding using too short of a hash function gives out an error" '
  echo foo | test_must_fail ipfs block put --mhlen 19 2>block_out
'

test_expect_success "error reason is pointed out" '
  grep "hashes must be at 20 least bytes long" block_out
'


test_cat_get() {

  test_expect_success "ipfs cat fails with unsafe hash function" '
    test_must_fail ipfs cat zDvnoLcPKWR 2>ipfs_cat
  '


  test_expect_success "error reason is pointed out" '
    grep "insecure hash functions not allowed" ipfs_cat
  '


  test_expect_success "ipfs get fails with too short function" '
    test_must_fail ipfs get z2ba5YhCCFNFxLtxMygQwjBjYSD8nUeN 2>ipfs_get

    '

  test_expect_success "error reason is pointed out" '
     grep "hashes must be at 20 least bytes long" ipfs_get
  '
}


test_gc() {
  test_expect_success "injecting insecure block" '
    mkdir -p "$IPFS_PATH/blocks/JZ" &&
    cp -f ../t0275-cid-security-data/AFKSEBCGPUJZE.data "$IPFS_PATH/blocks/JZ"
  '

  test_expect_success "gc works" 'ipfs repo gc > gc_out'
  test_expect_success "gc removed bad block" '
    grep zDvnoGUyhEq gc_out
  '
}


# should work offline
test_cat_get
test_gc

# should work online
test_launch_ipfs_daemon
test_cat_get
test_gc
test_kill_ipfs_daemon

test_done
