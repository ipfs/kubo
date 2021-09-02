#!/usr/bin/env bash
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test peerlog plugin"

. lib/test-lib.sh

test_expect_success "setup testbed" '
  iptb testbed create -type localipfs -count 2 -force -init
'

startup_cluster 2

test_expect_success "peerlog is disabled by default" '
  go-sleep 100ms
  iptb logs 0 >node0logs
  test_expect_code 1 grep peerlog node0logs
'

test_expect_success 'stop iptb' 'iptb stop'



test_expect_success "setup testbed" '
  iptb testbed create -type localipfs -count 2 -force -init
'

test_expect_success "enable peerlog config setting" '
  iptb run -- ipfs config --json Plugins.Plugins.peerlog.Config.Enabled true
'

startup_cluster 2

test_expect_success "peerlog plugin is logged" '
  go-sleep 100ms
  iptb logs 0 >node0logs
  grep peerlog node0logs
'

test_expect_success 'peer id' '
  PEERID_1=$(iptb attr get 1 id)
'

test_expect_success "peer id is logged" '
  iptb logs 0 | grep -q "$PEERID_1"
'

test_expect_success 'stop iptb' 'iptb stop'

test_done
