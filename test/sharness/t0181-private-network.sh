#!/usr/bin/env bash
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test private network feature"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "disable AutoConf for private network tests" '
  ipfs config --json AutoConf.Enabled false
'

export LIBP2P_FORCE_PNET=1

test_expect_success "daemon won't start with force pnet env but with no key" '
  test_must_fail go-timeout 5 ipfs daemon > stdout 2>&1
'

unset LIBP2P_FORCE_PNET

test_expect_success "daemon output includes info about the reason" '
  grep "private network was not configured but is enforced by the environment" stdout ||
  test_fsh cat stdout
'

pnet_key() {
  echo '/key/swarm/psk/1.0.0/'
  echo '/bin/'
  random-data -size=32
}

pnet_key > "${IPFS_PATH}/swarm.key"

LIBP2P_FORCE_PNET=1 test_launch_ipfs_daemon

test_expect_success "set up iptb testbed" '
  iptb testbed create -type localipfs -count 5 -force -init &&
  iptb run -- ipfs config --json "Routing.LoopbackAddressesOnLanDHT" true &&
  iptb run -- ipfs config --json "Swarm.Transports.Network.Websocket" false &&
  iptb run -- ipfs config --json Addresses.Swarm  '"'"'["/ip4/127.0.0.1/tcp/0"]'"'"' &&
  iptb run -- ipfs config --json AutoConf.Enabled false
'

set_key() {
  node="$1"
  keyfile="$2"

  cp "$keyfile" "${IPTB_ROOT}/testbeds/default/${node}/swarm.key"
}

pnet_key > key1
pnet_key > key2

set_key 1 key1
set_key 2 key1

set_key 3 key2
set_key 4 key2

unset LIBP2P_FORCE_PNET

test_expect_success "start nodes" '
  iptb start -wait [0-4]
'

test_expect_success "try connecting node in public network with priv networks" '
  test_must_fail iptb connect --timeout=2s [1-4] 0
'

test_expect_success "node 0 (public network) swarm is empty" '
  ipfsi 0 swarm peers &&
  [ $(ipfsi 0 swarm peers | wc -l) -eq 0 ]
'

test_expect_success "try connecting nodes in different private networks" '
  test_must_fail iptb connect 2 3
'

test_expect_success "node 3 (pnet 2) swarm is empty" '
  ipfsi 3 swarm peers &&
  [ $(ipfsi 3 swarm peers | wc -l) -eq 0 ]
'

test_expect_success "connect nodes in the same pnet" '
  iptb connect 1 2 &&
  iptb connect 3 4
'

test_expect_success "nodes 1 and 2 have connected" '
  ipfsi 2 swarm peers &&
  [ $(ipfsi 2 swarm peers | wc -l) -eq 1 ]
'

test_expect_success "nodes 3 and 4 have connected" '
  ipfsi 4 swarm peers &&
  [ $(ipfsi 4 swarm peers | wc -l) -eq 1 ]
'


run_single_file_test() {
  node1=$1
  node2=$2

  test_expect_success "add a file on node$node1" '
    random-data -size=1000000 > filea &&
    FILEA_HASH=$(ipfsi $node1 add -q filea)
  '

  check_file_fetch $node1 $FILEA_HASH filea
  check_file_fetch $node2 $FILEA_HASH filea
}

check_file_fetch() {
  node="$1"
  fhash="$2"
  fname="$3"

  test_expect_success "can fetch file" '
    ipfsi $node cat $fhash > fetch_out
  '

  test_expect_success "file looks good" '
    test_cmp $fname fetch_out
  '
}

run_single_file_test 1 2
run_single_file_test 2 1

run_single_file_test 3 4
run_single_file_test 4 3


test_expect_success "stop testbed" '
  iptb stop
'

test_kill_ipfs_daemon

# Test that AutoConf with default mainnet URL fails on private networks
test_expect_success "setup test repo with AutoConf enabled and private network" '
  export IPFS_PATH="$(pwd)/.ipfs-autoconf-test" &&
  ipfs init --profile=test > /dev/null &&
  ipfs config --json AutoConf.Enabled true &&
  pnet_key > "${IPFS_PATH}/swarm.key"
'

test_expect_success "daemon fails with AutoConf + private network error" '
  export IPFS_PATH="$(pwd)/.ipfs-autoconf-test" &&
  test_expect_code 1 ipfs daemon > autoconf_stdout 2> autoconf_stderr
'

test_expect_success "error message mentions AutoConf and private network conflict" '
  grep "AutoConf cannot use the default mainnet URL" autoconf_stderr > /dev/null &&
  grep "private network.*swarm.key" autoconf_stderr > /dev/null &&
  grep "AutoConf.Enabled=false" autoconf_stderr > /dev/null
'

test_done
