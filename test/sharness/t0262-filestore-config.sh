#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_enable_filestore

test_add_cat_file "filestore add" "`pwd`" "QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH"

export IPFS_LOGGING=debug
export IPFS_LOGGING_FMT=nocolor

HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH"

test_expect_success "file always checked" '
  ipfs config Filestore.Verify always 2> log &&
  ipfs cat "$HASH" 2> log &&
  grep -q "verifying block $HASH" log &&
  ! grep -q "updating block $HASH" log
'

test_expect_success "file checked after change" '
  ipfs config Filestore.Verify ifchanged 2> log &&
  sleep 2 && # to accommodate systems without sub-second mod-times
  echo "HELLO WORLDS!" >mountdir/hello.txt &&
  test_must_fail ipfs cat "$HASH" 2> log &&
  grep -q "verifying block $HASH" log &&
  grep -q "updating block $HASH" log
'

test_expect_success "file never checked" '
  echo "Hello Worlds!" >mountdir/hello.txt &&
  ipfs add "$dir"/mountdir/hello.txt >actual 2> log &&
  ipfs config Filestore.Verify never 2> log &&
  echo "HELLO Worlds!" >mountdir/hello.txt &&
  ( ipfs cat "$HASH" || true ) 2> log &&
  grep -q "BlockService GetBlock" log && # Make sure we are still logging
  ! grep -q "verifying block $HASH" log &&
  ! grep -q "updating block $HASH" log 
'

test_done
