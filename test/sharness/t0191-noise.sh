#!/usr/bin/env bash

test_description="Test ping over NOISE command"

. lib/test-lib.sh

test_init_ipfs

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count 3 -init
'

tcp_addr='"[\"/ip4/127.0.0.1/tcp/0\"]"'
test_expect_success "configure security transports" '
iptb run <<CMDS
  [0,1] -- ipfs config --json Swarm.Transports.Security.TLS false &&
  [0,1] -- ipfs config --json Swarm.Transports.Security.SECIO false &&
  2     -- ipfs config --json Swarm.Transports.Security.Noise false &&
        -- ipfs config --json Addresses.Swarm '${tcp_addr}'
CMDS
'

startup_cluster 2

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id)
'

test_expect_success "test ping other" '
  ipfsi 0 ping -n2 -- "$PEERID_1" &&
  ipfsi 1 ping -n2 -- "$PEERID_0"
'

test_expect_success "test tls incompatible" '
  iptb start --wait 2 &&
  test_must_fail iptb connect 2 0 > connect_error 2>&1 &&
  test_should_contain "failed to negotiate security protocol" connect_error ||
  test_fsh cat connect_error
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
