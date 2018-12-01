#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

num_nodes=4

test_expect_success "set up an iptb cluster" '
  iptb testbed create -type localipfs -count $num_nodes -force -init
'

startup_cluster $num_nodes

test_expect_success "add an obect on one node" '
  echo "ipns is super fun" > file &&
  HASH_FILE=$(ipfsi 1 add -q file)
'

test_expect_success "publish that object as an ipns entry" '
  ipfsi 1 name publish $HASH_FILE
'

test_expect_success "add an entry on another node pointing to that one" '
  NODE1_ID=$(iptb attr get 1 id) &&
  ipfsi 2 name publish /ipns/$NODE1_ID
'

test_expect_success "cat that entry on a third node" '
  NODE2_ID=$(iptb attr get 2 id) &&
  ipfsi 3 cat /ipns/$NODE2_ID > output
'

test_expect_success "ensure output was the same" '
  test_cmp file output
'

test_expect_success "shut down iptb" '
  iptb stop
'

test_done
