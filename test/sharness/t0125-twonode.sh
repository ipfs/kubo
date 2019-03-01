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

# Enable quic but don't use it yet.
test_expect_success "enable QUIC experiment" '
  ipfsi 0 config --json Experimental.QUIC true &&
  ipfsi 1 config --json Experimental.QUIC true
'

# test multiplex muxer
echo "Running advanced tests with mplex"
export LIBP2P_MUX_PREFS="/mplex/6.7.0"
run_advanced_test "--enable-mplex-experiment"
unset LIBP2P_MUX_PREFS

# test default configuration
echo "Running advanced tests with default config"
run_advanced_test

# test QUIC
echo "Running advanced tests over QUIC"
addr1='"[\"/ip4/127.0.0.1/udp/0/quic/\"]"'
addr2='"[\"/ip4/127.0.0.1/udp/0/quic/\"]"'
test_expect_success "add QUIC swarm addresses" '
  ipfsi 0 config --json Addresses.Swarm '$addr1' &&
  ipfsi 1 config --json Addresses.Swarm '$addr2'
'

run_advanced_test

test_done
