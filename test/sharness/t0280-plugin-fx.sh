#!/usr/bin/env bash

test_description="Test fx plugin"

. lib/test-lib.sh

test_init_ipfs

export GOLOG_LOG_LEVEL="fxtestplugin=debug"
export TEST_FX_PLUGIN=1
test_launch_ipfs_daemon

test_expect_success "expected log entry should be present" '
  fgrep "invoked test fx function" daemon_err >/dev/null
'

test_kill_ipfs_daemon

test_done
