#!/usr/bin/env bash

test_description="Test reprovider"

. lib/test-lib.sh

NUM_NODES=2

test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -force -count $NUM_NODES -init
'

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id)
'

test_expect_success 'use strategic providing' '
  iptb run -- ipfs config --json Experimental.StrategicProviding true
'

startup_cluster ${NUM_NODES}

test_expect_success 'add test object' '
    HASH_0=$(echo "foo" | ipfsi 0 add -q)
'

findprovs_empty '$HASH_0'

test_expect_success 'stop node 1' '
  iptb stop
'

test_done
