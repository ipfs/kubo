#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

echo "currently skipping 'Test daemon command', until we find a better way to wait."
exit 0

test_description="Test daemon command"

. lib/test-lib.sh

# NOTE: this should remove bootstrap peers (needs a flag)
test_expect_success "ipfs daemon --init launches" '
  export IPFS_DIR="$(pwd)/.go-ipfs" &&
  ipfs daemon --init 2>&1 >actual_init &
'

# this is because we have no way of knowing the daemon is done except look at
# output. but we can't yet compare it because we dont have the peer ID (config)
test_expect_success "initialization ended" '
  IPFS_PID=$! &&
  test_wait_output_n_lines_60_sec actual_init 6
'

# this is lifted straight from t0020-init.sh
test_expect_success "ipfs peer id looks good" '
  PEERID=$(ipfs config Identity.PeerID) &&
  echo $PEERID | tr -dC "[:alnum:]" | wc -c | tr -d " " >actual &&
  echo "46" >expected &&
  test_cmp_repeat_10_sec expected actual
'

# note this is almost the same as t0020-init.sh "ipfs init output looks good"
test_expect_success "ipfs daemon output looks good" '
  STARTHASH="QmYpv2VEsxzTTXRYX3PjDg961cnJE3kY1YDXLycHGQ3zZB" &&
  echo "initializing ipfs node at $IPFS_DIR" >expected &&
  echo "generating key pair...done" >>expected &&
  echo "peer identity: $PEERID" >>expected &&
  echo "\nto get started, enter: ipfs cat $STARTHASH" >>expected &&
  echo "daemon listening on /ip4/127.0.0.1/tcp/5001" >>expected &&
  test_cmp_repeat_10_sec expected actual_init
'

test_expect_success ".go-ipfs/ has been created" '
  test -d ".go-ipfs" &&
  test -f ".go-ipfs/config" &&
  test -d ".go-ipfs/datastore" ||
  fsh ls .go-ipfs
'

test_expect_success "daemon is still running" '
  kill -0 $IPFS_PID
'

test_expect_success "'ipfs daemon' can be killed" '
  test_kill_repeat_10_sec $IPFS_PID
'

test_done
