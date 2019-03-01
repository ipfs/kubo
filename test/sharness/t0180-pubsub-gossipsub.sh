#!/usr/bin/env bash

test_description="Test pubsub with gossipsub"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count $NUM_NODES -init
'

test_expect_success "enable gossipsub" '
  for x in $(seq 0 4); do
    ipfsi $x config Pubsub.Router gossipsub
  done
'

# this is just a copy of t0180-pubsub; smell.
startup_cluster $NUM_NODES --enable-pubsub-experiment

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_2=$(iptb attr get 2 id)
'

test_expect_success 'pubsub' '
  echo "testOK" > expected &&
  touch empty &&
  mkfifo wait ||
  test_fsh echo init fail

  # ipfs pubsub sub is long-running so we need to start it in the background and
  # wait put its output somewhere where we can access it
  (
    ipfsi 0 pubsub sub --enc=ndpayload testTopic | if read line; then
        echo $line > actual &&
        echo > wait
      fi
  ) &
'

test_expect_success "wait until ipfs pubsub sub is ready to do work" '
  go-sleep 500ms
'

test_expect_success "can see peer subscribed to testTopic" '
  ipfsi 1 pubsub peers testTopic > peers_out
'

test_expect_success "output looks good" '
  echo $PEERID_0 > peers_exp &&
  test_cmp peers_exp peers_out
'

test_expect_success "publish something" '
  ipfsi 1 pubsub pub testTopic "testOK" &> pubErr
'

test_expect_success "wait until echo > wait executed" '
  cat wait &&
  test_cmp pubErr empty &&
  test_cmp expected actual
'

test_expect_success "wait for another pubsub message" '
  echo "testOK2" > expected &&
  mkfifo wait2 ||
  test_fsh echo init fail

  # ipfs pubsub sub is long-running so we need to start it in the background and
  # wait put its output somewhere where we can access it
  (
    ipfsi 2 pubsub sub --enc=ndpayload testTopic | if read line; then
        echo $line > actual &&
        echo > wait2
      fi
  ) &
'

test_expect_success "wait until ipfs pubsub sub is ready to do work" '
  go-sleep 500ms
'

test_expect_success "publish something" '
  echo "testOK2" | ipfsi 1 pubsub pub testTopic &> pubErr
'

test_expect_success "wait until echo > wait executed" '
  echo "testOK2" > expected &&
  cat wait2 &&
  test_cmp pubErr empty &&
  test_cmp expected actual
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
