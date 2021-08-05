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

test_expect_success "test profiling (without CPU)" '
  ipfs diag profile --cpu-profile-time=0 > cmd_out
'

test_expect_success "filename shows up in output" '
  grep -q "ipfs-profile" cmd_out > /dev/null
'

test_expect_success "profile file created" '
  test -e "$(sed -n -e "s/.*\(ipfs-profile.*\.zip\)/\1/p" cmd_out)"
'

test_expect_success "test profiling with -o" '
  ipfs diag profile --cpu-profile-time=1s -o test-profile.zip
'

test_expect_success "test that test-profile.zip exists" '
  test -e test-profile.zip
'
test_kill_ipfs_daemon

if ! test_have_prereq UNZIP; then
    test_done
fi

test_expect_success "unpack profiles" '
  unzip -d profiles test-profile.zip
'

test_expect_success "cpu profile is valid" '
  go tool pprof -top profiles/ipfs "profiles/cpu.pprof" | grep -q "Type: cpu"
'

test_expect_success "heap profile is valid" '
  go tool pprof -top profiles/ipfs "profiles/heap.pprof" | grep -q "Type: inuse_space"
'

test_expect_success "goroutines profile is valid" '
  go tool pprof -top profiles/ipfs "profiles/goroutines.pprof" | grep -q "Type: goroutine"
'

test_expect_success "goroutines stacktrace is valid" '
  grep -q "goroutine" "profiles/goroutines.stacks"
'

test_done
