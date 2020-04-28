#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test symlink support on the HTTP gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon


test_expect_success "Create a test directory with symlinks" '
  mkdir testfiles &&
  echo "content" > testfiles/foo &&
  ln -s foo testfiles/bar &&
  test_cmp testfiles/foo testfiles/bar
'

test_expect_success "Add the test directory" '
  HASH=$(ipfs add -Qr testfiles)
'

test_expect_success "Test the directory listing" '
  curl "$GWAY_ADDR/ipfs/$HASH" > list_response &&
  test_should_contain ">foo<" list_response &&
  test_should_contain ">bar<" list_response
'

test_expect_success "Test the symlink" '
  curl "$GWAY_ADDR/ipfs/$HASH/bar" > bar_actual &&
  echo -n "foo" > bar_expected &&
  test_cmp bar_expected bar_actual
'

test_kill_ipfs_daemon

test_done
