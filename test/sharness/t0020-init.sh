#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_PATH="$(pwd)/.ipfs" &&
	BITS="2048" &&
	ipfs init --bits="$BITS" >actual_init
'

test_expect_success ".ipfs/ has been created" '
	test -d ".ipfs" &&
	test -f ".ipfs/config" &&
	test -d ".ipfs/datastore" ||
	test_fsh ls -al .ipfs
'

test_expect_success "ipfs config succeeds" '
	echo leveldb >expected_config &&
	ipfs config Datastore.Type >actual_config &&
	test_cmp expected_config actual_config
'

test_expect_success "ipfs peer id looks good" '
	PEERID=$(ipfs config Identity.PeerID) &&
	echo $PEERID | tr -dC "[:alnum:]" | wc -c | tr -d " " >actual_peerid &&
	echo "46" >expected_peerid &&
	test_cmp expected_peerid actual_peerid
'

test_expect_success "ipfs init output looks good" '
	STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&
	echo "initializing ipfs node at $IPFS_PATH" >expected &&
	echo "generating $BITS-bit RSA keypair...done" >>expected &&
	echo "peer identity: $PEERID" >>expected &&
	echo "to get started, enter:" >>expected &&
	printf "\\n\\t$STARTFILE\\n\\n" >>expected &&
	test_cmp expected actual_init
'

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success "ipfs init should not run while daemon is running" '
	test_must_fail ipfs init 2> daemon_running_err &&
	EXPECT="Error: ipfs daemon is running. please stop it to run this command" &&
	grep "$EXPECT" daemon_running_err
'

test_kill_ipfs_daemon

test_done
