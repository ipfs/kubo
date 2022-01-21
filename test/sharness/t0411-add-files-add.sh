#!/usr/bin/env bash
#
# Copyright (c) 2016 Tom O'Donnell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test files add and add commands"

. lib/test-lib.sh

test_init_ipfs


test_launch_ipfs_daemon

# Verify CIDS are the same
test_expect_success "files add CID matches add CID" '
  DIR="/test-${RANDOM}"
	ipfs files mkdir $DIR

  echo "hi" > f1
	ADDCID=$(ipfs add -q f1)
	FILESADDCID=$(ipfs files add -q $DIR f1)
	test_cmp ADDCID FILESADDCID
'

# verify files add does not result in a pin.
test_expect_code 1 '
  DIR="/test-${RANDOM}"
	ipfs files mkdir $DIR

	echo "${RANDOM}" > f2
	CID=$(ipfs files add -q $DIR/ f2)

	ipfs pin ls | grep $CID
'
test_kill_ipfs_daemon

test_done
