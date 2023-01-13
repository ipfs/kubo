#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test two ipfs nodes transferring a file"

. lib/test-lib.sh

check_file_fetch() {
  node=$1
  fhash=$2
  fname=$3

  test_expect_success "can fetch file" '
    ipfsi $node cat $fhash > fetch_out
  '

  test_expect_success "file looks good" '
    test_cmp $fname fetch_out
  '
}

check_dir_fetch() {
  node=$1
  ref=$2

  test_expect_success "node can fetch all refs for dir" '
    ipfsi $node refs -r $ref > /dev/null
  '
}

run_single_file_test() {
  test_expect_success "add a file on node1" '
    random 1000000 > filea &&
    FILEA_HASH=$(ipfsi 1 add -q filea)
  '

  check_file_fetch 0 $FILEA_HASH filea
}

run_random_dir_test() {
  test_expect_success "create a bunch of random files" '
    random-files -depth=3 -dirs=4 -files=5 -seed=5 foobar > /dev/null
  '

  test_expect_success "add those on node 0" '
    DIR_HASH=$(ipfsi 0 add -r -Q foobar)
  '

  check_dir_fetch 1 $DIR_HASH
}

flaky_advanced_test() {
  startup_cluster 2 "$@"

  test_expect_success "clean repo before test" '
    ipfsi 0 repo gc > /dev/null &&
    ipfsi 1 repo gc > /dev/null
  '

  run_single_file_test

  run_random_dir_test

  test_expect_success "gather bitswap stats" '
    ipfsi 0 bitswap stat -v > stat0 &&
    ipfsi 1 bitswap stat -v > stat1
  '

  test_expect_success "shut down nodes" '
    iptb stop && iptb_wait_stop
  '

  # NOTE: data transferred stats checks are flaky
  #       trying to debug them by printing out the stats hides the flakiness
  #       my theory is that the extra time cat calls take to print out the stats
  #       allow for proper cleanup to happen
  go-sleep 1s
}

run_advanced_test() {
  # TODO: investigate why flaky_advanced_test is flaky
  # Context: https://github.com/ipfs/kubo/pull/9486
  # sometimes, bitswap status returns  unexpected block transfers
  # and everyone has been re-running circleci until is passes for at least a year.
  # this re-runs test until it passes or a timeout hits

  BLOCKS_0=126
  BLOCKS_1=5
  DATA_0=228113
  DATA_1=1000256
  for i in $(test_seq 1 600); do
    flaky_advanced_test
    (grep -q "$DATA_0" stat0 && grep -q "$DATA_1" stat1) && break
    go-sleep 100ms
  done

  test_expect_success "node0 data transferred looks correct" '
    test_should_contain "blocks sent: $BLOCKS_0" stat0 &&
    test_should_contain "blocks received: $BLOCKS_1" stat0 &&
    test_should_contain "data sent: $DATA_0" stat0 &&
    test_should_contain "data received: $DATA_1" stat0
  '

  test_expect_success "node1 data transferred looks correct" '
    test_should_contain "blocks received: $BLOCKS_0" stat1 &&
    test_should_contain "blocks sent: $BLOCKS_1" stat1 &&
    test_should_contain "data received: $DATA_0" stat1 &&
    test_should_contain "data sent: $DATA_1" stat1
  '

}

test_expect_success "set up tcp testbed" '
  iptb testbed create -type localipfs -count 2 -force -init
'

test_expect_success "disable routing, use direct peering" '
  iptb run -- ipfs config Routing.Type none &&
  iptb run -- ipfs config --json Bootstrap "[]"
'

# Test TCP transport
echo "Testing TCP"
addrs='"[\"/ip4/127.0.0.1/tcp/0\"]"'
test_expect_success "use TCP only" '
  iptb run -- ipfs config --json Addresses.Swarm '"${addrs}"' &&
  iptb run -- ipfs config --json Swarm.Transports.Network.QUIC false &&
  iptb run -- ipfs config --json Swarm.Transports.Network.Relay false &&
  iptb run -- ipfs config --json Swarm.Transports.Network.WebTransport false &&
  iptb run -- ipfs config --json Swarm.Transports.Network.Websocket false
'
run_advanced_test

# test multiplex muxer
echo "Running TCP tests with mplex"
test_expect_success "disable yamux" '
  iptb run -- ipfs config --json Swarm.Transports.Multiplexers.Yamux false
'
run_advanced_test

test_expect_success "re-enable yamux" '
  iptb run -- ipfs config --json Swarm.Transports.Multiplexers.Yamux null
'
# test Noise
echo "Running TCP tests with NOISE"
test_expect_success "use noise only" '
  iptb run -- ipfs config --json Swarm.Transports.Security.TLS false
'
run_advanced_test

test_expect_success "re-enable TLS" '
  iptb run -- ipfs config --json Swarm.Transports.Security.TLS null
'

# test QUIC
echo "Running advanced tests over QUIC"
addrs='"[\"/ip4/127.0.0.1/udp/0/quic-v1\"]"'
test_expect_success "use QUIC only" '
  iptb run -- ipfs config --json Addresses.Swarm '"${addrs}"' &&
  iptb run -- ipfs config --json Swarm.Transports.Network.QUIC true &&
  iptb run -- ipfs config --json Swarm.Transports.Network.TCP false
'
run_advanced_test

# test WebTransport
echo "Running advanced tests over WebTransport"
addrs='"[\"/ip4/127.0.0.1/udp/0/quic-v1/webtransport\"]"'
test_expect_success "use WebTransport only" '
  iptb run -- ipfs config --json Addresses.Swarm '"${addrs}"' &&
  iptb run -- ipfs config --json Swarm.Transports.Network.QUIC true &&
  iptb run -- ipfs config --json Swarm.Transports.Network.WebTransport true
'
run_advanced_test

test_done
