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
    DIR_HASH=$(ipfsi 0 add -r -q foobar | tail -n1)
  '

  check_dir_fetch 1 $DIR_HASH
}

run_advanced_test() {
  startup_cluster 2 "$@"

  test_expect_success "clean repo before test" '
    ipfsi 0 repo gc > /dev/null &&
    ipfsi 1 repo gc > /dev/null
  '

  run_single_file_test

  run_random_dir_test

  test_expect_success "node0 data transferred looks correct" '
    ipfsi 0 bitswap stat > stat0 &&
    grep "blocks sent: 126" stat0 > /dev/null &&
    grep "blocks received: 5" stat0 > /dev/null &&
    grep "data sent: 228113" stat0 > /dev/null &&
    grep "data received: 1000256" stat0 > /dev/null
  '

  test_expect_success "node1 data transferred looks correct" '
    ipfsi 1 bitswap stat > stat1 &&
    grep "blocks received: 126" stat1 > /dev/null &&
    grep "blocks sent: 5" stat1 > /dev/null &&
    grep "data received: 228113" stat1 > /dev/null &&
    grep "data sent: 1000256" stat1 > /dev/null
  '

  test_expect_success "shut down nodes" '
    iptb stop && iptb_wait_stop
  '
}

test_expect_success "set up tcp testbed" '
  iptb testbed create -type localipfs -count 2 -force -init
'

addrs='"[\"/ip4/127.0.0.1/tcp/0\", \"/ip4/127.0.0.1/udp/0/quic\"]"'
test_expect_success "configure addresses" '
  ipfsi 0 config --json Addresses.Swarm '"${addrs}"' &&
  ipfsi 1 config --json Addresses.Swarm '"${addrs}"'
'

# Test TCP transport
echo "Testing TCP"
test_expect_success "use TCP only" '
  iptb run -- ipfs config --json Swarm.Transports.Network.QUIC false &&
  iptb run -- ipfs config --json Swarm.Transports.Network.Relay false &&
  iptb run -- ipfs config --json Swarm.Transports.Network.Websocket false
'
run_advanced_test

# test multiplex muxer
echo "Running advanced tests with mplex"
test_expect_success "disable yamux" '
  iptb run -- ipfs config --json Swarm.Transports.Multiplexers.Yamux false
'
run_advanced_test

test_expect_success "re-enable yamux" '
  iptb run -- ipfs config --json Swarm.Transports.Multiplexers.Yamux null
'

# test Noise

echo "Running advanced tests with NOISE"
test_expect_success "use noise only" '
  iptb run -- ipfs config --json Swarm.Transports.Security.TLS false &&
  iptb run -- ipfs config --json Swarm.Transports.Security.Secio false
'

run_advanced_test

# test QUIC
echo "Running advanced tests over QUIC"
test_expect_success "use QUIC only" '
  iptb run -- ipfs config --json Swarm.Transports.Network.QUIC true &&
  iptb run -- ipfs config --json Swarm.Transports.Network.TCP false
'

run_advanced_test

test_done
