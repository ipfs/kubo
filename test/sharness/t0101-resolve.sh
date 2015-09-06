#!/bin/sh
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs resolve operations"

. lib/test-lib.sh

test_init_ipfs

# setup

test_expect_success "setup: add files" '
	mkdir mountdir/foodir &&
	echo "Hello Mars!" >mountdir/foodir/mars.txt &&
	echo "/ipfs/`ipfs add -q mountdir/foodir/mars.txt`" >filehash &&
	ipfs add -q -r mountdir/foodir | tail -n 1 >dirhash
'

test_expect_success "setup: get IPFS ID" '
	ID=`ipfs id --format="<id>\n"`
'

# resolve an IPNS name to a simple /ipfs/ path

test_expect_success "setup: publish file to IPNS name" '
	ipfs name publish `cat filehash`
'

test_expect_success "'ipfs resolve' succeeds with IPNS->path" '
	echo `ipfs resolve "/ipns/$ID"` >actual
'

test_expect_success "'ipfs resolve' output looks good" '
	test_cmp filehash actual
'

# resolve an IPNS name to a /ipfs/HASH/link/name path

test_expect_success "setup: publish link path to IPNS name" '
	ipfs name publish "`cat dirhash`/mars.txt"
'

test_expect_success "'ipfs resolve' succeeds with IPNS->link path" '
	echo `ipfs resolve "/ipns/$ID"` >actual
'

test_expect_success "'ipfs resolve' output looks good" '
	test_cmp filehash actual
'


test_done
