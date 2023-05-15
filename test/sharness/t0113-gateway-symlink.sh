#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test symlink support on the HTTP gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

# Import test case
# See the static fixtures in ./t0113-gateway-symlink/
test_expect_success "Add the test directory with symlinks" '
  ipfs dag import ../t0113-gateway-symlink/testfiles.car
'
ROOT_DIR_CID=QmWvY6FaqFMS89YAQ9NAPjVP4WZKA1qbHbicc9HeSKQTgt # ./testfiles/

test_expect_success "Test the directory listing" '
  curl "$GWAY_ADDR/ipfs/$ROOT_DIR_CID/" > list_response &&
  test_should_contain ">foo<" list_response &&
  test_should_contain ">bar<" list_response
'

test_expect_success "Test the symlink" '
  curl "$GWAY_ADDR/ipfs/$ROOT_DIR_CID/bar" > bar_actual &&
  echo -n "foo" > bar_expected &&
  test_cmp bar_expected bar_actual
'

test_kill_ipfs_daemon

test_done
