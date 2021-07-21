#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test profile collection"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "profiling requires a running daemon" '
  test_must_fail ipfs diag profile
'

test_launch_ipfs_daemon

test_expect_success "test profiling" '
  ipfs diag profile --cpu-profile-time=1s > cmd_out
'

test_expect_success "filename shows up in output" '
  grep -q "ipfs-profile" cmd_out > /dev/null
'

test_expect_success "profile file created" '
  test -e "$(sed -n -e "s/.*\(ipfs-profile.*\.zip\)/\1/p" cmd_out)"
'

test_expect_success "test profiling with -o (without CPU profiling)" '
  ipfs diag profile --cpu-profile-time=0 -o test-profile.zip
'

test_expect_success "test that test-profile.zip exists" '
  test -e test-profile.zip
'

test_kill_ipfs_daemon
test_done
