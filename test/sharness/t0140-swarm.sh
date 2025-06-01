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


announceCfg='["/ip4/127.0.0.1/tcp/4001", "/ip4/1.2.3.4/tcp/1234"]'
test_expect_success "test_config_set succeeds" "
  ipfs config --json Addresses.Announce '$announceCfg'
"
# Include "/ip4/1.2.3.4/tcp/1234" to ensure we deduplicate addrs already present in Swarm.Announce
appendAnnounceCfg='["/dnsaddr/dynamic.example.com", "/ip4/10.20.30.40/tcp/4321", "/ip4/1.2.3.4/tcp/1234"]'
test_expect_success "test_config_set Announce and AppendAnnounce succeeds" "
  ipfs config --json Addresses.Announce '$announceCfg' &&
  ipfs config --json Addresses.AppendAnnounce '$appendAnnounceCfg'
"

test_launch_ipfs_daemon

test_expect_success 'Addresses.AppendAnnounce is applied on top of Announce' '
  ipfs swarm addrs local >actual &&
  grep "/ip4/1.2.3.4/tcp/1234" actual &&
  grep "/dnsaddr/dynamic.example.com" actual &&
  grep "/ip4/10.20.30.40/tcp/4321" actual &&
  ipfs id -f"<addrs>" | xargs -n1 echo | tee actual &&
  grep "/ip4/1.2.3.4/tcp/1234/p2p" actual &&
  grep "/dnsaddr/dynamic.example.com/p2p/" actual &&
  grep "/ip4/10.20.30.40/tcp/4321/p2p/" actual
'

test_kill_ipfs_daemon

noAnnounceCfg='["/ip4/1.2.3.4/tcp/1234"]'
test_expect_success "test_config_set succeeds" "
  ipfs config --json Addresses.NoAnnounce '$noAnnounceCfg'
"

test_launch_ipfs_daemon

test_expect_success "Addresses.NoAnnounce affects addresses from Announce and AppendAnnounce" '
  ipfs swarm addrs local >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual &&
  grep -v "/ip4/10.20.30.40/tcp/4321" actual &&
  ipfs id -f"<addrs>" | xargs -n1 echo >actual &&
  grep -v "/ip4/1.2.3.4/tcp/1234" actual &&
  grep -v "//ip4/10.20.30.40/tcp/4321" actual
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

test_launch_ipfs_daemon

test_expect_success "'ipfs swarm peering ls' lists peerings" '
  ipfs swarm peering ls
'

peeringID='QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N'
peeringID2='QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5K'
peeringAddr='/ip4/1.2.3.4/tcp/1234/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N'
peeringAddr2='/ip4/1.2.3.4/tcp/1234/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5K'
test_expect_success "'ipfs swarm peering add' adds a peering" '
  ipfs swarm peering ls > peeringls &&
  ! test_should_contain ${peeringID} peeringls &&
  ! test_should_contain ${peeringID2} peeringls &&
  ipfs swarm peering add ${peeringAddr} ${peeringAddr2}
'

test_expect_success 'a peering is added' '
  ipfs swarm peering ls > peeringadd &&
  test_should_contain ${peeringID} peeringadd &&
  test_should_contain ${peeringID2} peeringadd
'

test_expect_success "'swarm peering rm' removes a peering" '
  ipfs swarm peering rm ${peeringID}
'

test_expect_success 'peering is removed' '
  ipfs swarm peering ls > peeringrm &&
  ! test_should_contain ${peeringID} peeringrm
'

test_kill_ipfs_daemon

test_launch_ipfs_daemon

peeringID='QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N'
peeringID2='QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5K'
peeringAddr='/ip4/1.2.3.4/tcp/1234/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N'
peeringAddr2='/ip4/1.2.3.4/tcp/1234/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5K'

test_expect_success "'ipfs swarm peering add' with save option mutates config" '
  ipfs config Peering.Peers > start-peers &&
  ipfs swarm peering add ${peeringAddr} ${peeringAddr2} --save &&
  ipfs config Peering.Peers > end-peers &&
  ! test_cmp start-peers end-peers &&
  test_should_contain "${peeringID}" end-peers &&
  test_should_contain ${peeringID2} end-peers &&
  rm start-peers end-peers
'

test_expect_success "'ipfs swarm peering rm' with save option mutates config" '
  ipfs config Peering.Peers > start-peers &&
  ipfs swarm peering rm ${peeringID} --save &&
  ipfs config Peering.Peers > end-peers &&
  ! test_cmp start-peers end-peers &&
  test_should_not_contain "${peeringID}" end-peers &&
  test_should_contain ${peeringID2} end-peers &&
  rm start-peers end-peers
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
