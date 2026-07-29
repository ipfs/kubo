#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test active request commands"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

# By default the daemon drops every finished request from the log each time the
# log reaches a multiple of ten entries. The polling below makes several
# requests, so without this the entry we are waiting for could be swept away
# before we see it.
test_expect_success "keep finished requests in the log" '
  ipfs diag cmds set-time 60s
'

test_expect_success "command works" '
  ipfs diag cmds > cmd_out
'

test_expect_success "invoc shows up in output" '
  grep "diag/cmds" cmd_out > /dev/null
'

test_expect_success "start longer running command" '
  ipfs log tail &
  LOGPID=$!
'

# The daemon only lists the request once the backgrounded client has connected,
# which on a loaded machine takes longer than any fixed sleep we could pick.
test_expect_success "long running command shows up" '
  test_run_repeat_60_sec "ipfs diag cmds > cmd_out2 && grep log/tail cmd_out2 | grep true"
'

test_expect_success "output looks good" '
  grep "log/tail" cmd_out2 | grep "true" > /dev/null
'

test_expect_success "kill log cmd" '
  kill $LOGPID
  go-sleep 0.5s
  kill $LOGPID

  wait $LOGPID || true
'

# Same on the way out: the daemon marks the request inactive when it notices the
# client is gone, not when kill returns.
test_expect_success "long running command inactive" '
  test_run_repeat_60_sec "ipfs diag cmds > cmd_out3 && grep log/tail cmd_out3 | grep false"
'

test_expect_success "command shows up as inactive" '
  grep "log/tail" cmd_out3 | grep "false"
'

test_kill_ipfs_daemon
test_done
