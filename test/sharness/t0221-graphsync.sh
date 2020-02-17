#!/usr/bin/env bash

test_description="Test fetching from graphsync."

# imports
. lib/test-lib.sh

test_init_ipfs

test_expect_success 'configuring ipfs' '
  ipfs config --json Experimental.GraphsyncEnabled true
'

test_expect_success 'add content' '
  HASH=$(random 1000000 | ipfs add -q)
'

test_launch_ipfs_daemon

test_expect_success 'get addrs' '
  ADDR="$(ipfs id --format="<addrs>" | head -1)"
'

test_expect_success 'fetch' '
  graphsync-get "$ADDR" "$HASH" > result
'

test_expect_success 'check' '
  ipfs add -q < result > hash_actual &&
  echo "$HASH" > hash_expected &&
  test_cmp hash_expected hash_actual
'

test_kill_ipfs_daemon
test_done
