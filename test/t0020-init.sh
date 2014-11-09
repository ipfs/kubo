#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_DIR="$(pwd)/.go-ipfs" &&
	ipfs init
'

test_expect_success ".go-ipfs/ has been created" '
	test -d ".go-ipfs" &&
	test -f ".go-ipfs/config" &&
	test -d ".go-ipfs/datastore"
'

test_expect_success "ipfs config succeeds" '
	echo leveldb >expected &&
	ipfs config Datastore.Type >actual &&
	test_cmp expected actual
'

test_done

