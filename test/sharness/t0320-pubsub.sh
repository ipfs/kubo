#!/usr/bin/env bash

test_description="Test pubsub command"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count $NUM_NODES -init
'

test_expect_success 'disable the DHT' '
  iptb run -- ipfs config Routing.Type none
'

run_pubsub_tests() {
  test_expect_success 'peer ids' '
    PEERID_0=$(iptb attr get 0 id) &&
    PEERID_2=$(iptb attr get 2 id)
  '

  # ipfs pubsub sub
  test_expect_success 'pubsub' '
    echo -n -e "test\nOK" | ipfs multibase encode -b base64url > expected &&
    touch empty &&
    mkfifo wait ||
    test_fsh echo init fail

    # ipfs pubsub sub is long-running so we need to start it in the background and
    # wait put its output somewhere where we can access it
    (
      ipfsi 0 pubsub sub --enc=json testTopic | if read line; then
          echo $line | jq -j .data > actual &&
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

  test_expect_success "publish something from file" '
    echo -n -e "test\nOK" > payload-file &&
    ipfsi 1 pubsub pub testTopic payload-file &> pubErr
  '

  test_expect_success "wait until echo > wait executed" '
    cat wait &&
    test_cmp pubErr empty &&
    test_cmp expected actual
  '

  test_expect_success "wait for another pubsub message" '
    echo -n -e "test\nOK\r\n2" | ipfs multibase encode -b base64url > expected &&
    mkfifo wait2 ||
    test_fsh echo init fail

    # ipfs pubsub sub is long-running so we need to start it in the background and
    # wait put its output somewhere where we can access it
    (
      ipfsi 2 pubsub sub --enc=json testTopic | if read line; then
          echo $line | jq -j .data > actual &&
          echo > wait2
        fi
    ) &
  '

  test_expect_success "wait until ipfs pubsub sub is ready to do work" '
    go-sleep 500ms
  '

  test_expect_success "publish something from stdin" '
    echo -n -e "test\nOK\r\n2" | ipfsi 3 pubsub pub testTopic &> pubErr
  '

  test_expect_success "wait until echo > wait executed" '
    cat wait2 &&
    test_cmp pubErr empty &&
    test_cmp expected actual
  '

  test_expect_success 'cleanup fifos' '
    rm -f wait wait2
  '

}

# Normal tests - enabled via config

test_expect_success 'enable the pubsub' '
  iptb run -- ipfs config --json Pubsub.Enabled true
'

startup_cluster $NUM_NODES
run_pubsub_tests
test_expect_success 'stop iptb' '
  iptb stop
'

test_expect_success 'disable the pubsub' '
  iptb run -- ipfs config --json Pubsub.Enabled false
'

# Normal tests - enabled via daemon option flag

startup_cluster $NUM_NODES --enable-pubsub-experiment
run_pubsub_tests
test_expect_success 'stop iptb' '
  iptb stop
'

# Test with some nodes not signing messages.

test_expect_success 'disable signing on nodes 1-3' '
  iptb run [0-3] -- ipfs config --json Pubsub.DisableSigning true
'

startup_cluster $NUM_NODES --enable-pubsub-experiment

test_expect_success 'set node 4 to listen on testTopic' '
  rm -f node4_actual &&
  ipfsi 4 pubsub sub --enc=json testTopic > node4_actual &
'

run_pubsub_tests

test_expect_success 'stop iptb' '
  iptb stop
'

test_expect_success 'node 4 got no unsigned messages' '
  test_must_be_empty node4_actual
'


# Confirm negative CLI flag takes precedence over positive config

# --enable-pubsub-experiment=false + Pubsub.Enabled:true

test_expect_success 'enable the pubsub via config' '
  iptb run -- ipfs config --json Pubsub.Enabled true
'
startup_cluster $NUM_NODES --enable-pubsub-experiment=false

test_expect_success 'pubsub cmd fails because it was disabled via cli flag' '
  test_expect_code 1 ipfsi 4 pubsub ls 2> pubsub_cmd_out
'

test_expect_success "pubsub cmd produces error" '
  echo "Error: experimental pubsub feature not enabled. Run daemon with --enable-pubsub-experiment to use." > expected &&
  test_cmp expected pubsub_cmd_out
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
