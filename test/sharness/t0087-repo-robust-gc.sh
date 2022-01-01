#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test robustness of garbage collector"

. lib/test-lib.sh
set -e

to_raw_cid() {
    ipfs cid format -b b --codec raw -v 1 "$1"
}

test_gc_robust_part1() {

  test_expect_success "add a 1MB file with --raw-leaves" '
    random 1048576 56 > afile &&
    HASH1=`ipfs add --raw-leaves -q --cid-version 1 afile` &&
    REFS=`ipfs refs -r $HASH1` &&
    read LEAF1 LEAF2 LEAF3 LEAF4 < <(echo $REFS)
  '

  test_expect_success "find data blocks for added file" '
    HASH1MH=`cid-fmt -b base32 "%M" $HASH1` &&
    LEAF1MH=`cid-fmt -b base32 "%M" $LEAF1` &&
    LEAF2MH=`cid-fmt -b base32 "%M" $LEAF2` &&
    HASH1FILE=`find .ipfs/blocks -type f | grep -i $HASH1MH` &&
    LEAF1FILE=`find .ipfs/blocks -type f | grep -i $LEAF1MH` &&
    LEAF2FILE=`find .ipfs/blocks -type f | grep -i $LEAF2MH`
  '

  test_expect_success "remove a leaf node from the repo manually" '
    rm "$LEAF1FILE"
  '

 test_expect_success "check that the node is removed" '
   test_must_fail ipfs cat $HASH1
 '

  test_expect_success "'ipfs repo gc' should still be fine" '
    ipfs repo gc
  '

  test_expect_success "corrupt the root node of 1MB file" '
    test -e "$HASH1FILE" &&
    dd if=/dev/zero of="$HASH1FILE" count=1 bs=100 conv=notrunc
  '

  test_expect_success "'ipfs repo gc' should abort without removing anything" '
    test_must_fail ipfs repo gc 2>&1 | tee gc_err &&
    grep -q "could not retrieve links for $HASH1" gc_err &&
    grep -q "aborted" gc_err
  '

  test_expect_success "leaf nodes were not removed after gc" '
    ipfs cat $LEAF3 > /dev/null &&
    ipfs cat $LEAF4 > /dev/null
  '

  test_expect_success "unpin the 1MB file" '
    ipfs pin rm $HASH1
  '

  # make sure the permission problem is fixed on exit, otherwise cleanup
  # will fail
  trap "chmod 700 `dirname "$LEAF2FILE"` 2> /dev/null || true" 0

  test_expect_success "create a permission problem" '
    chmod 500 `dirname "$LEAF2FILE"` &&
    test_must_fail ipfs block rm $LEAF2 2>&1 | tee block_rm_err &&
    grep -q "permission denied" block_rm_err
  '

  # repo gc outputs raw multihashes. We chech HASH1 with block stat rather than
  # grepping the output since it's not a raw multihash
  test_expect_success "'ipfs repo gc' should still run and remove as much as possible" '
    test_must_fail ipfs repo gc 2>&1 | tee repo_gc_out &&
    grep -q "could not remove $LEAF2" repo_gc_out &&
    grep -q "removed $(to_raw_cid $LEAF3)" repo_gc_out &&
    grep -q "removed $(to_raw_cid $LEAF4)" repo_gc_out &&
    test_must_fail ipfs block stat $HASH1
  '

  test_expect_success "fix the permission problem" '
    chmod 700 `dirname "$LEAF2FILE"`
  '

  test_expect_success "'ipfs repo gc' should be ok now" '
    ipfs repo gc | tee repo_gc_out
    grep -q "removed $(to_raw_cid $LEAF2)" repo_gc_out
  '
}

test_gc_robust_part2() {

  test_expect_success "add 1MB file normally (i.e., without raw leaves)" '
    random 1048576 56 > afile &&
    HASH2=`ipfs add -q afile`
  '

  LEAF1=QmSijovevteoY63Uj1uC5b8pkpDU5Jgyk2dYBqz3sMJUPc
  LEAF1FILE=.ipfs/blocks/ME/CIQECF2K344QITW5S6E6H6T4DOXDDB2XA2V7BBOCIMN2VVF4Q77SMEY.data

  LEAF2=QmTbPEyrA1JyGUHFvmtx1FNZVzdBreMv8Hc8jV9sBRWhNA
  LEAF2FILE=.ipfs/blocks/WM/CIQE4EFIJN2SUTQYSKMKNG7VM75W3SXT6LWJCHJJ73UAWN73WCX3WMY.data


  test_expect_success "add some additional unpinned content" '
    random 1000 3 > junk1 &&
    random 1000 4 > junk2 &&
    JUNK1=`ipfs add --pin=false -q junk1` &&
    JUNK2=`ipfs add --pin=false -q junk2`
  '

  test_expect_success "remove a leaf node from the repo manually" '
    rm "$LEAF1FILE"
  '

  test_expect_success "'ipfs repo gc' should abort" '
    test_must_fail ipfs repo gc 2>&1 | tee repo_gc_out &&
    grep -q "could not retrieve links for $LEAF1" repo_gc_out &&
    grep -q "aborted" repo_gc_out
  '

  test_expect_success "test that garbage collector really aborted" '
    ipfs cat $JUNK1 > /dev/null &&
    ipfs cat $JUNK2 > /dev/null 
  '

  test_expect_success "corrupt a key" '
    test -e "$LEAF2FILE" &&
    dd if=/dev/zero of="$LEAF2FILE" count=1 bs=100 conv=notrunc
  '

  test_expect_success "'ipfs repo gc' should abort with two errors" '
    test_must_fail ipfs repo gc 2>&1 | tee repo_gc_out &&
    grep -q "could not retrieve links for $LEAF1" repo_gc_out &&
    grep -q "could not retrieve links for $LEAF2" repo_gc_out &&
    grep -q "aborted" repo_gc_out
  '

  test_expect_success "'ipfs repo gc --stream-errors' should abort and report each error separately" '
    test_must_fail ipfs repo gc --stream-errors 2>&1 | tee repo_gc_out &&
    grep -q "Error: could not retrieve links for $LEAF1" repo_gc_out &&
    grep -q "Error: could not retrieve links for $LEAF2" repo_gc_out &&
    grep -q "Error: garbage collection aborted" repo_gc_out
  '

  test_expect_success "unpin 1MB file" '
    ipfs pin rm $HASH2
  '

  test_expect_success "'ipfs repo gc' should be fine now" '
    ipfs repo gc | tee repo_gc_out &&
    grep -q "removed $(to_raw_cid $HASH2)" repo_gc_out &&
    grep -q "removed $(to_raw_cid $LEAF2)" repo_gc_out
  '
}

test_init_ipfs

test_gc_robust_part1
test_gc_robust_part2

test_launch_ipfs_daemon_without_network

test_gc_robust_part1
test_gc_robust_part2

test_kill_ipfs_daemon

test_done

