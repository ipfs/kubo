#!/usr/bin/env bash

test_description="Test ping command"

. lib/test-lib.sh

test_init_ipfs

BAD_PEER="QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJx"

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count 2 -init
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

test_expect_success "test ping unreachable peer" '
  printf "Looking up peer %s\n" "$BAD_PEER" > bad_ping_exp &&
  printf "Error: peer lookup failed: routing: not found\n" >> bad_ping_exp &&
  ! ipfsi 0 ping -n2 -- "$BAD_PEER" > bad_ping_actual 2>&1 &&
  test_cmp bad_ping_exp bad_ping_actual
'

test_expect_success "test ping self" '
  ! ipfsi 0 ping -n2 -- "$PEERID_0" &&
  ! ipfsi 1 ping -n2 -- "$PEERID_1"
'

test_expect_success "test ping 0" '
  ! ipfsi 0 ping -n0 -- "$PEERID_1" &&
  ! ipfsi 1 ping -n0 -- "$PEERID_0"
'

test_expect_success "test ping offline" '
  iptb stop 1 &&
  ! ipfsi 0 ping -n2 -- "$PEERID_1"
'

test_expect_success 'stop iptb' '
  iptb stop 0
'

test_done
