#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command with default config"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "Swarm.NatPortMap set to true in inital config" '
	echo "true" > expected &&
	ipfs config Swarm.NatPortMap > actual &&
	test_cmp expected actual
'

test_expect_success "Swarm.NatPortMap has line in config file" '
	grep -q NatPortMap .ipfs/config
'

test_expect_success "remove Swarm.NatPortMap from config file" '
	sed -i "s/NatPortMap/XxxYyyyZzz/" .ipfs/config
'

test_expect_success "load config file by replacing a unrelated key" '
	ipfs config --json Swarm.DisableBandwidthMetrics false
'

test_expect_success "Swarm.NatPortMap set to true in new config" '
	echo "true" > expected &&
	ipfs config Swarm.NatPortMap > actual &&
	test_cmp expected actual
'

test_expect_success "reset Swarm group to default values" '
	ipfs config --json Swarm {}
'

test_expect_success "Swarm.NatPortMap still set to true in reset config" '
	echo "true" > expected &&
	ipfs config Swarm.NatPortMap > actual &&
	test_cmp expected actual
'

test_done
