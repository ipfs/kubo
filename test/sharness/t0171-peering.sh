#!/usr/bin/env bash

test_description="Test peering service"

. lib/test-lib.sh

NUM_NODES=3

test_expect_success 'init iptb' '
  rm -rf .iptb/ &&
  iptb testbed create -type localipfs -count $NUM_NODES -init
'

test_expect_success 'disabling routing' '
  iptb run -- ipfs config Routing.Type none
'

for i in $(seq 0 2); do
  ADDR="$(printf '["/ip4/127.0.0.1/tcp/%s"]' "$(( 3000 + ( RANDOM % 1000 ) ))")"
  test_expect_success "configuring node $i to listen on $ADDR" '
    ipfsi "$i" config --json Addresses.Swarm "$ADDR"
  '
done

peer_id() {
    ipfsi "$1" config Identity.PeerID
}

peer_addrs() {
    ipfsi "$1" config Addresses.Swarm
}

peer() {
  PEER1="$1" &&
  PEER2="$2" &&
  PEER_LIST="$(ipfsi "$PEER1" config Peering.Peers)" &&
  { [[ "$PEER_LIST" == "null" ]] || PEER_LIST_INNER="${PEER_LIST:1:-1}"; } &&
  ADDR_INFO="$(printf '[%s{"ID": "%s", "Addrs": %s}]' \
             "${PEER_LIST_INNER:+${PEER_LIST_INNER},}" \
             "$(peer_id "$PEER2")" \
             "$(peer_addrs "$PEER2")")" &&
  ipfsi "$PEER1" config --json Peering.Peers "${ADDR_INFO}"
}

# Peer:
# - 0 <-> 1
# - 1 -> 2
test_expect_success 'configure peering' '
  peer 0 1 &&
  peer 1 0 &&
  peer 1 2
'

list_peers() {
    ipfsi "$1" swarm peers | sed 's|.*/p2p/\([^/]*\)$|\1|' | sort -u
}

check_peers() {
  sleep 20 # give it some time to settle.
  test_expect_success 'verifying peering for peer 0' '
    list_peers 0 > peers_0_actual &&
    peer_id 1 > peers_0_expected &&
    test_cmp peers_0_expected peers_0_actual
  '

  test_expect_success 'verifying peering for peer 1' '
    list_peers 1 > peers_1_actual &&
    { peer_id 0 && peer_id 2 ; } | sort -u > peers_1_expected &&
    test_cmp peers_1_expected peers_1_actual
  '

  test_expect_success 'verifying peering for peer 2' '
    list_peers 2 > peers_2_actual &&
    peer_id 1 > peers_2_expected &&
    test_cmp peers_2_expected peers_2_actual
  '
}

test_expect_success 'startup cluster' '
  iptb start -wait &&
  iptb run -- ipfs log level peering debug
'

check_peers

disconnect() {
    ipfsi "$1" swarm disconnect "/p2p/$(peer_id "$2")"
}

# Bidirectional peering shouldn't cause problems (e.g., simultaneous connect
# issues).
test_expect_success 'disconnecting 0->1' '
  disconnect 0 1
'

check_peers

# 1 should reconnect to 2 when 2 disconnects from 1.
test_expect_success 'disconnecting 2->1' '
  disconnect 2 1
'

check_peers

# 2 isn't peering. This test ensures that 1 will re-peer with 2 when it comes
# back online.
test_expect_success 'stopping 2' '
  iptb stop 2
'

# Wait to disconnect
sleep 30

test_expect_success 'starting 2' '
  iptb start 2
'

# Wait for backoff
sleep 30

check_peers

test_expect_success "stop testbed" '
  iptb stop
'

test_done
