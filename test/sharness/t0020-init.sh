#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

# test that ipfs fails to init if IPFS_PATH isnt writeable
test_expect_success "create dir and change perms succeeds" '
	export IPFS_PATH="$(pwd)/.badipfs" &&
	mkdir "$IPFS_PATH" &&
	chmod 000 "$IPFS_PATH"
'

test_expect_success "ipfs init fails" '
	test_must_fail ipfs init 2> init_fail_out
'

test_expect_success "ipfs init output looks good" '
	echo "Error: failed to take lock at $IPFS_PATH: permission denied" > init_fail_exp &&
	test_cmp init_fail_exp init_fail_out
'

# test no repo error message
# this applies to `ipfs add sth`, `ipfs refs <hash>`
test_expect_success "ipfs cat fails" '
    export IPFS_PATH="$(pwd)/.ipfs" &&
    test_must_fail ipfs cat Qmaa4Rw81a3a1VEx4LxB7HADUAXvZFhCoRdBzsMZyZmqHD 2> cat_fail_out
'

test_expect_success "ipfs cat no repo message looks good" '
    echo "Error: no ipfs repo found in $IPFS_PATH." > cat_fail_exp &&
    echo "please run: ipfs init" >> cat_fail_exp &&
    test_cmp cat_fail_exp cat_fail_out
'

# test that init succeeds
test_expect_success "ipfs init succeeds" '
	export IPFS_PATH="$(pwd)/.ipfs" &&
	BITS="2048" &&
	ipfs init --bits="$BITS" >actual_init
'

test_expect_success ".ipfs/ has been created" '
	test -d ".ipfs" &&
	test -f ".ipfs/config" &&
	test -d ".ipfs/datastore" &&
	test -d ".ipfs/blocks" ||
	test_fsh ls -al .ipfs
'

test_expect_success "ipfs config succeeds" '
	echo /ipfs >expected_config &&
	ipfs config Mounts.IPFS >actual_config &&
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

test_expect_success "clean up ipfs dir" '
	rm -rf "$IPFS_PATH"
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
