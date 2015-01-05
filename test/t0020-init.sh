#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_DIR="$(pwd)/.go-ipfs" &&
	ipfs init >actual_init
'

test_expect_success ".go-ipfs/ has been created" '
	test -d ".go-ipfs" &&
	test -f ".go-ipfs/config" &&
	test -d ".go-ipfs/datastore" ||
	fsh ls -al .go-ipfs
'

test_expect_success "ipfs config succeeds" '
	echo leveldb >expected &&
	ipfs config Datastore.Type >actual &&
	test_cmp expected actual
'

test_expect_success "ipfs peer id looks good" '
	PEERID=$(ipfs config Identity.PeerID) &&
	echo $PEERID | tr -dC "[:alnum:]" | wc -c | tr -d " " >actual &&
	echo "46" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs init output looks good" '
	STARTHASH="QmYpv2VEsxzTTXRYX3PjDg961cnJE3kY1YDXLycHGQ3zZB" &&
	echo "initializing ipfs node at $IPFS_DIR" >expected &&
	echo "generating key pair...done" >>expected &&
	echo "peer identity: $PEERID" >>expected &&
	echo "\nto get started, enter: ipfs cat $STARTHASH" >>expected &&
	test_cmp expected actual_init
'

test_done
