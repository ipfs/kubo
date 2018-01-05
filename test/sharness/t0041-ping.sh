#!/bin/sh

test_description="Test ping command"

. lib/test-lib.sh

test_init_ipfs

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb init -n 2 --bootstrap=none --port=0
'

startup_cluster 2

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_1=$(iptb get id 1)
'

test_expect_success "test ping other" '
  ipfsi 0 ping -n2 -- "$PEERID_1" &&
  ipfsi 1 ping -n2 -- "$PEERID_0"
'

test_expect_success "test ping self" '
  ! ipfsi 0 ping -n2 -- "$PEERID_0" &&
  ! ipfsi 1 ping -n2 -- "$PEERID_1"
'

test_expect_success "test ping 0" '
  ! ipfsi 0 ping -n0 -- "$PEERID_1" &&
  ! ipfsi 1 ping -n0 -- "$PEERID_0"
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
