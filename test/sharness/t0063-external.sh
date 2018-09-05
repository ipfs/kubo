#!/usr/bin/env bash
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test external command functionality"

. lib/test-lib.sh


# set here so daemon launches with it
PATH=`pwd`/bin:$PATH

test_init_ipfs

test_expect_success "create fake ipfs-update bin" '
  mkdir bin &&
  echo "#!/bin/sh" > bin/ipfs-update &&
  echo "pwd" >> bin/ipfs-update &&
  echo "test -e \"$IPFS_PATH/repo.lock\" || echo \"repo not locked\" " >> bin/ipfs-update &&
  chmod +x bin/ipfs-update &&
  mkdir just_for_test
'

test_expect_success "external command runs from current user directory and doesn't lock repo" '
  (cd just_for_test && ipfs update) > actual
'

test_expect_success "output looks good" '
  echo `pwd`/just_for_test > exp &&
  echo "repo not locked" >> exp &&
  test_cmp exp actual
'

test_launch_ipfs_daemon

test_expect_success "external command runs from current user directory when daemon is running" '
  (cd just_for_test && ipfs update) > actual
'

test_expect_success "output looks good" '
  echo `pwd`/just_for_test > exp &&
  test_cmp exp actual
'

test_kill_ipfs_daemon

test_done
