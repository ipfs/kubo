#!/bin/sh
#
# Copyright (c) 2016 Mike Pfister 
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo fsck operations"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "'ipfs repo fsck' succeeds with no daemon running" '
    echo ":D" >> $IPFS_PATH/repo.lock &&
    mkdir -p $IPFS_PATH/datastore &&
    touch $IPFS_PATH/datastore/LOCK &&
    ipfs repo fsck > fsck_out_actual1
'
test_expect_success "'ipfs repo fsck' output looks good with no daemon" '
  grep "Lockfiles have been removed." fsck_out_actual1
'

# Make sure the files are actually removed
test_expect_success "'ipfs repo fsck' confirm file deletion" '
  test ! -e "$IPFS_PATH/repo.lock" &&
  test ! -e "$IPFS_PATH/datastore/LOCK"
'

# daemon is not running and partial lock exists for either repo.lock or LOCK 
test_expect_success "'ipfs repo fsck' succeeds partial lock" '
    touch $IPFS_PATH/datastore/LOCK &&
    ipfs repo fsck > fsck_out_actual2
'

test_expect_success "'ipfs repo fsck' output looks good with no daemon" '
  grep "Lockfiles have been removed." fsck_out_actual2
'

# Make sure the files are actually removed
test_expect_success "'ipfs repo fsck' confirm file deletion" '
  test ! -e "$IPFS_PATH/repo.lock" &&
  test ! -e "$IPFS_PATH/datastore/LOCK"
'

test_expect_success "'ipfs repo fsck' succeeds partial lock" '
    echo ":D" >> $IPFS_PATH/repo.lock &&
    ipfs repo fsck > fsck_out_actual3
'
test_expect_success "'ipfs repo fsck' output looks good with no daemon" '
  grep "Lockfiles have been removed." fsck_out_actual3
'

# Make sure the files are actually removed
test_expect_success "'ipfs repo fsck' confirm file deletion" '
  test ! -e "$IPFS_PATH/repo.lock" &&
  test ! -e "$IPFS_PATH/datastore/LOCK"
'

# TODO: test ipfs/api exists but address inside points to a dead daemon
test_expect_success "'ipfs repo fsck' api removed when pointing to dead daemon" '
  echo "/ip4/127.0.0.1/tcp/1111" > $IPFS_PATH/api &&
  ipfs repo fsck > fsck_out_actual4
'

test_expect_success "'ipfs repo fsck' output looks good with no daemon" '
  grep "Lockfiles have been removed." fsck_out_actual4
'

# Make sure the files are actually removed
test_expect_success "'ipfs repo fsck' confirm file deletion" '
  test ! -e "$IPFS_PATH/repo.lock" &&
  test ! -e "$IPFS_PATH/datastore/LOCK"
'
test_launch_ipfs_daemon

# Daemon is running -> command doesn't run
test_expect_success "'ipfs repo fsck' fails with daemon running" '
  ipfs repo fsck > fsck_out_actual5
'

test_expect_success "'ipfs repo fsck' output looks good with daemon" '
  grep "Error: ipfs daemon is running" fsck_out_actual5
'

# Daemon is not running but repo.lock exists and has a non-zero size.
# command still runs and cleans both files
test_kill_ipfs_daemon

test_done
