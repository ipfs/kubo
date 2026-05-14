#!/usr/bin/env bash

test_description="Test circuit relay"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=3
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count $NUM_NODES -init &&
  iptb run -- ipfs config --json "Routing.LoopbackAddressesOnLanDHT" true
'

# Network topology: A <-> Relay <-> B
test_expect_success 'start up nodes for configuration' '
  iptb start -wait -- --routing=none
'

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id) &&
  PEERID_2=$(iptb attr get 2 id)
'

relayaddrs=$(ipfsi 1 swarm addrs local | jq --raw-input . | jq --slurp .)
staticrelay=$(ipfsi 1 swarm addrs local | sed -e "s|$|/p2p/$PEERID_1|g" | jq --raw-input . | jq --slurp .)

test_expect_success 'configure the relay node as a static relay for node A' '
    ipfsi 0 config Internal.Libp2pForceReachability private &&
    ipfsi 0 config --json Swarm.RelayClient.Enabled true &&
    ipfsi 0 config --json Swarm.RelayClient.StaticRelays "$staticrelay"
'

test_expect_success 'configure the relay node' '
  ipfsi 1 config Internal.Libp2pForceReachability public &&
  ipfsi 1 config --json Swarm.RelayService.Enabled true &&
  ipfsi 1 config --json Addresses.Swarm "$relayaddrs"
'

test_expect_success 'configure the node B' '
    ipfsi 2 config Internal.Libp2pForceReachability private &&
    ipfsi 2 config --json Swarm.RelayClient.Enabled true
'

test_expect_success 'restart nodes' '
  iptb stop &&
  iptb_wait_stop &&
  iptb start -wait -- --routing=none
'

test_expect_success 'connect A <-> Relay' '
  iptb connect 0 1
'

test_expect_success 'connect B <-> Relay' '
  iptb connect 2 1
'

test_expect_success 'wait until relay is ready to do work' '
  while ! ipfsi 2 swarm connect /p2p/$PEERID_1/p2p-circuit/p2p/$PEERID_0; do
    iptb stop &&
    iptb_wait_stop &&
    iptb start -wait -- --routing=none &&
    iptb connect 0 1 &&
    iptb connect 2 1 &&
    sleep 5
  done
'

test_expect_success 'connect A <-Relay-> B' '
  ipfsi 2 swarm connect /p2p/$PEERID_1/p2p-circuit/p2p/$PEERID_0 > peers_out
'

test_expect_success 'output looks good' '
  echo "connect $PEERID_0 success" > peers_exp &&
  test_cmp peers_exp peers_out
'

test_expect_success 'peers for A look good' '
  ipfsi 0 swarm peers > peers_out &&
  test_should_contain "/p2p/$PEERID_1/p2p-circuit/p2p/$PEERID_2$" peers_out
'

test_expect_success 'peers for B look good' '
  ipfsi 2 swarm peers > peers_out &&
  test_should_contain "/p2p/$PEERID_1/p2p-circuit/p2p/$PEERID_0$" peers_out
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
