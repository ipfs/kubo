#!/bin/sh
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "GET IPFS path succeeds" '
  echo "Hello Worlds!" > expected &&
  HASH=`ipfs add -q expected` &&
  wget http://127.0.0.1:5001/ipfs/"$HASH" -O actual
'

test_expect_success "GET IPFS path output looks good" '
  test_cmp expected actual &&
  rm actual
'

test_expect_success "GET IPFS directory path succeeds" '
  mkdir dir &&
  echo "12345" > dir/test &&
  HASH2=`ipfs add -r -q dir | tail -n 1` &&
  wget http://127.0.0.1:5001/ipfs/"$HASH2"
'

test_expect_success "GET IPFS directory file succeeds" '
  wget http://127.0.0.1:5001/ipfs/"$HASH2"/test -O actual
'

test_expect_success "GET IPFS directory file output looks good" '
  test_cmp dir/test actual
'

test_expect_failure "GET IPNS path succeeds" '
  ipfs name publish "$HASH" &&
  NAME=`ipfs config Identity.PeerID` &&
  wget http://127.0.0.1:5001/ipns/"$NAME" -O actual
'

test_expect_failure "GET IPNS path output looks good" '
  test_cmp expected actual
'

test_expect_success "GET invalid IPFS path errors" '
  test_must_fail wget http://127.0.0.1:5001/ipfs/12345
'

test_expect_success "GET invalid path errors" '
  test_must_fail wget http://127.0.0.1:5001/12345
'

test_kill_ipfs_daemon

test_done
