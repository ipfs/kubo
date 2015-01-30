#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_PATH="$(pwd)/.go-ipfs" &&
	ipfs init >actual_init
'

test_expect_success ".go-ipfs/ has been created" '
	test -d ".go-ipfs" &&
	test -f ".go-ipfs/config" &&
	test -d ".go-ipfs/datastore" ||
	fsh ls -al .go-ipfs
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
	STARTHASH="QmPXME1oRtoT627YKaDPDQ3PwA8tdP9rWuAAweLzqSwAWT" &&
	STARTFILE="ipfs cat /ipfs/$STARTHASH/readme"
	echo "initializing ipfs node at $IPFS_PATH" >expected &&
	echo "generating 4096-bit RSA keypair...done" >>expected &&
	echo "peer identity: $PEERID" >>expected &&
	echo "to get started, enter:" >>expected &&
	printf "\\n\\t$STARTFILE\\n\\n" >>expected &&
	test_cmp expected actual_init
'

test_done
