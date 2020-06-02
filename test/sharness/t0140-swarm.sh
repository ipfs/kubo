#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs swarm command"

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success 'disconnected: peers is empty' '
  ipfs swarm peers >actual &&
  test_must_be_empty actual
'

test_expect_success 'disconnected: addrs local has localhost' '
  ipfs swarm addrs local >actual &&
  grep "/ip4/127.0.0.1" actual
'

test_expect_success 'disconnected: addrs local matches ipfs id' '
  ipfs id -f="<addrs>\\n" | sort >expected &&
  ipfs swarm addrs local --id | sort >actual &&
  test_cmp expected actual
'

test_expect_success "ipfs id self works" '
  myid=$(ipfs id -f="<id>") &&
  ipfs id --timeout=1s $myid > output
'

test_expect_success "output looks good" '
  grep $myid output &&
  grep PublicKey output
'

addr="/ip4/127.0.0.1/tcp/9898/p2p/QmUWKoHbjsqsSMesRC2Zoscs8edyFz6F77auBB1YBBhgpX"

test_expect_success "can't trigger a dial backoff with swarm connect" '
  test_expect_code 1 ipfs swarm connect $addr 2> connect_out
  test_expect_code 1 ipfs swarm connect $addr 2>> connect_out
  test_expect_code 1 ipfs swarm connect $addr 2>> connect_out
  test_expect_code 1 grep "backoff" connect_out
'

test_kill_ipfs_daemon

announceCfg='["/ip4/127.0.0.1/tcp/4001", "/ip4/1.2.3.4/tcp/1234"]'
test_expect_success "test_config_set succeeds" "
  ipfs config --json Addresses.Announce '$announceCfg'
"

test_launch_ipfs_daemon

test_expect_success 'Addresses.Announce affects addresses' '
  ipfs swarm addrs local >actual &&
  grep "/ip4/1.2.3.4/tcp/1234" actual &&
  ipfs id -f"<addrs>" | xargs -n1 echo >actual &&
  grep "/ip4/1.2.3.4/tcp/1234" actual
'

test_kill_ipfs_daemon

noAnnounceCfg='["/ip4/1.2.3.4/tcp/1234"]'
test_expect_success "test_config_set succeeds" "
  ipfs config --json Addresses.NoAnnounce '$noAnnounceCfg'
"

test_launch_ipfs_daemon

test_expect_success "Addresses.NoAnnounce affects addresses" '
  ipfs swarm addrs local >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual &&
  ipfs id -f"<addrs>" | xargs -n1 echo >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual
'

test_kill_ipfs_daemon

noAnnounceCfg='["/ip4/1.2.3.4/ipcidr/16"]'
test_expect_success "test_config_set succeeds" "
  ipfs config --json Addresses.NoAnnounce '$noAnnounceCfg'
"

test_launch_ipfs_daemon

test_expect_success "Addresses.NoAnnounce with /ipcidr affects addresses" '
  ipfs swarm addrs local >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual &&
  ipfs id -f"<addrs>" | xargs -n1 echo >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual
'

test_kill_ipfs_daemon

test_expect_success "set up tcp testbed" '
  iptb testbed create -type localipfs -count 2 -force -init
'

startup_cluster 2

test_expect_success "disconnect work without specifying a transport address" '
  [ $(ipfsi 0 swarm peers | wc -l) -eq 1 ] &&
  ipfsi 0 swarm disconnect "/p2p/$(iptb attr get 1 id)" &&
  [ $(ipfsi 0 swarm peers | wc -l) -eq 0 ]
'

test_expect_success "connect work without specifying a transport address" '
  [ $(ipfsi 0 swarm peers | wc -l) -eq 0 ] &&
  ipfsi 0 swarm connect "/p2p/$(iptb attr get 1 id)" &&
  [ $(ipfsi 0 swarm peers | wc -l) -eq 1 ]
'

test_expect_success "/p2p addresses work" '
  [ $(ipfsi 0 swarm peers | wc -l) -eq 1 ] &&
  ipfsi 0 swarm disconnect "/p2p/$(iptb attr get 1 id)" &&
  [ $(ipfsi 0 swarm peers | wc -l) -eq 0 ] &&
  ipfsi 0 swarm connect "/p2p/$(iptb attr get 1 id)" &&
  [ $(ipfsi 0 swarm peers | wc -l) -eq 1 ]
'

test_expect_success "ipfs id is consistent for node 0" '
  ipfsi 1 id "$(iptb attr get 0 id)" > 1see0 &&
  ipfsi 0 id > 0see0 &&
  test_cmp 1see0 0see0
'

test_expect_success "ipfs id is consistent for node 1" '
  ipfsi 0 id "$(iptb attr get 1 id)" > 0see1 &&
  ipfsi 1 id > 1see1 &&
  test_cmp 0see1 1see1
'

test_expect_success "addresses contain /p2p/..." '
  test_should_contain "/p2p/$(iptb attr get 1 id)\"" 0see1 &&
  test_should_contain "/p2p/$(iptb attr get 1 id)\"" 1see1 &&
  test_should_contain "/p2p/$(iptb attr get 0 id)\"" 1see0 &&
  test_should_contain "/p2p/$(iptb attr get 0 id)\"" 0see0
'

test_expect_success "stopping cluster" '
  iptb stop
'

test_done
