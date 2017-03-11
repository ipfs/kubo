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
	grep -q NatPortMap "$IPFS_PATH"/config
'

test_expect_success "remove Swarm.NatPortMap from config file" '
	sed -i "s/NatPortMap/XxxYyyyZzz/" "$IPFS_PATH"/config
'

test_expect_success "load config file by replacing an unrelated key" '
	# Note: this is relying on an implementation detail where setting a
	#   config field loads the json blob into the config struct, causing
	#   defaults to be properly set. Simply "reading" the config would read
	#   the json directly and not cause defaults to be reset
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
