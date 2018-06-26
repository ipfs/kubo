#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test out the urlstore functionality"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "enable urlstore" '
  ipfs config --json Experimental.UrlstoreEnabled true
'

test_expect_success "create some random files" '
  random 2222     7 > file1 &&
  random 50000000 7 > file2 
'

test_expect_success "add files using trickle dag format without raw leaves" '
  HASH1a=$(ipfs add -q --trickle --raw-leaves=false file1) &&
  HASH2a=$(ipfs add -q --trickle --raw-leaves=false file2)
'
test_launch_ipfs_daemon --offline

test_expect_success "make sure files can be retrived via the gateway" '
  curl http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a -o file1.actual &&
  test_cmp file1 file1.actual &&
  curl http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a -o file2.actual &&
  test_cmp file2 file2.actual 
'

test_expect_success "add files using gateway address via url store" '
  HASH1=$(ipfs urlstore add http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a) &&
  HASH2=$(ipfs urlstore add http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a)
'

test_expect_success "make sure hashes are different" '
  echo $HASH1a $HASH1
  echo $HASH2a $HASH2
'

test_expect_success "get files via urlstore" '
  ipfs get $HASH1 -o file1.actual &&
  test_cmp file1 file1.actual &&
  ipfs get $HASH2 -o file2.actual &&
  test_cmp file2 file2.actual
'

test_expect_success "remove original hashes from local gateway" '
  ipfs pin rm $HASH1a $HASH2a &&
  ipfs repo gc
'

test_expect_success "gatway no longer has files" '
  test_must_fail curl -f http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a -o file1.actual
  test_must_fail curl -f http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a -o file2.actual
'

test_expect_success "files can not be retrieved via the urlstore" '
  test_must_fail ipfs get $HASH1
  test_must_fail ipfs get $HASH2
'

test_kill_ipfs_daemon

test_done
