#!/usr/bin/env bash
#
# Copyright (c) 2020 Protocol Labs
# MIT/Apache-2.0 Licensed; see the LICENSE file in this repository.
#

test_description="Test swarm filters are effective"

AF="/ip4/127.0.0.0/ipcidr/24"

. lib/test-lib.sh

NUM_NODES=3

test_expect_success "set up testbed" '
  iptb testbed create -type localipfs -count $NUM_NODES -force -init &&
  iptb run -- ipfs config --json "Routing.LoopbackAddressesOnLanDHT" true
'

test_expect_success 'filter 127.0.0.0/24 on node 1' '
  ipfsi 1 config --json Swarm.AddrFilters "[\"$AF\"]"
'

for i in $(seq 0 $(( NUM_NODES - 1 ))); do
  test_expect_success "change IP for node $i" '
    ipfsi $i config --json "Addresses.Swarm" \
      "[\"/ip4/127.0.$i.1/tcp/0\",\"/ip4/127.0.$i.1/udp/0/quic\",\"/ip4/127.0.$i.1/tcp/0/ws\"]"
  '
done

test_expect_success 'start cluster' '
  iptb start --wait
'

test_expect_success 'connecting 1 to 0 fails' '
  test_must_fail iptb connect 1 0
'

test_expect_success 'connecting 0 to 1 fails' '
  test_must_fail iptb connect 1 0
'

test_expect_success 'connecting 2 to 0 succeeds' '
  iptb connect 2 0
'

test_expect_success 'connecting 1 to 0 with dns addrs fails' '
  ipfsi 0 id -f "<addrs>" | sed "s|^/ip4/127.0.0.1/|/dns4/localhost/|" > addrs &&
  test_must_fail ipfsi 1 swarm connect $(cat addrs)
'


test_expect_success 'stopping cluster' '
  iptb stop
'

test_done
