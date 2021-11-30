#!/usr/bin/env bash
#
# Copyright (c) 2021 Protocol Labs
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command with sharding enabled"

. lib/test-lib.sh

if ! test_have_prereq FUSE; then
  skip_all='skipping mount sharded tests, fuse not available'
  test_done
fi

test_init_ipfs

test_expect_success 'force sharding' '
  ipfs config --json Internal.UnixFSShardingSizeThreshold "\"1B\""
'

test_launch_ipfs_daemon
test_mount_ipfs

# we're testing nested subdirs which ensures that IPLD ADLs work
test_expect_success 'setup test data' '
  mkdir testdata &&
  echo a > testdata/a &&
  mkdir testdata/subdir &&
  echo b > testdata/subdir/b
'

HASH=QmY59Ufw8zA2BxGPMTcfXg86JVed81Qbxeq5rDkHWSLN1m

test_expect_success 'can add the data' '
  echo $HASH > expected_hash &&
  ipfs add -r -Q testdata > actual_hash &&
  test_cmp expected_hash actual_hash
'

test_expect_success 'can read the data' '
  echo a > expected_a &&
  cat "ipfs/$HASH/a" > actual_a &&
  test_cmp expected_a actual_a &&
  echo b > expected_b &&
  cat "ipfs/$HASH/subdir/b" > actual_b &&
  test_cmp expected_b actual_b
'

test_expect_success 'can list directories' '
  printf "a\nsubdir\n" > expected_ls &&
  ls -1 "ipfs/$HASH" > actual_ls &&
  test_cmp expected_ls actual_ls &&
  printf "b\n" > expected_ls_subdir &&
  ls -1 "ipfs/$HASH/subdir" > actual_ls_subdir &&
  test_cmp expected_ls_subdir actual_ls_subdir
'

test_expect_success "unmount" '
  do_umount "$(pwd)/ipfs" &&
  do_umount "$(pwd)/ipns"
'

test_expect_success 'cleanup' 'rmdir ipfs ipns'

test_kill_ipfs_daemon

test_done
