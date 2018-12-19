#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

export DEBUG=true

setup_iptb() {
  num_nodes="$1"
  bound=$(expr "$num_nodes" - 1)

  test_expect_success "iptb init" '
    iptb testbed create -type localipfs -count $num_nodes -init
  '

  for i in $(test_seq 0 "$bound")
  do
    test_expect_success "set configs up for node $i" '
      ipfsi "$i" config Ipns.RepublishPeriod 40s &&
      ipfsi "$i" config --json Ipns.ResolveCacheSize 0
    '
  done

  startup_cluster "$num_nodes"
}

teardown_iptb() {
  test_expect_success "shut down nodes" '
    iptb stop
  '
}

verify_can_resolve() {
  num_nodes="$1"
  bound=$(expr "$num_nodes" - 1)
  name="$2"
  expected="$3"
  msg="$4"

  for node in $(test_seq 0 "$bound")
  do
    test_expect_success "$msg: node $node can resolve entry" '
      ipfsi "$node" name resolve "$name" > resolve
    '

    test_expect_success "$msg: output for node $node looks right" '
      printf "/ipfs/$expected\n" > expected &&
      test_cmp expected resolve
    '
  done
}

verify_cannot_resolve() {
  num_nodes="$1"
  bound=$(expr "$num_nodes" - 1)
  name="$2"
  msg="$3"

  for node in $(test_seq 0 "$bound")
  do
    test_expect_success "$msg: resolution fails on node $node" '
      test_expect_code 1 ipfsi "$node" name resolve "$name"
    '
  done
}

num_test_nodes=4

setup_iptb "$num_test_nodes"

test_expect_success "publish succeeds" '
  HASH=$(echo "foobar" | ipfsi 1 add -q) &&
  ipfsi 1 name publish -t 10s $HASH
'

test_expect_success "get id succeeds" '
  id=$(ipfsi 1 id -f "<id>")
'

verify_can_resolve "$num_test_nodes" "$id" "$HASH" "just after publishing"

go-sleep 10s

verify_cannot_resolve "$num_test_nodes" "$id" "after 10 seconds, records are invalid"

go-sleep 30s

verify_can_resolve "$num_test_nodes" "$id" "$HASH" "republisher fires after 30 seconds"

#

test_expect_success "generate new key" '
KEY2=`ipfsi 1 key gen beepboop --type ed25519`
'

test_expect_success "publish with new key succeeds" '
  HASH=$(echo "barfoo" | ipfsi 1 add -q) &&
  ipfsi 1 name publish -t 10s -k "$KEY2" $HASH
'

verify_can_resolve "$num_test_nodes" "$KEY2" "$HASH" "new key just after publishing"

go-sleep 10s

verify_cannot_resolve "$num_test_nodes" "$KEY2" "new key cannot resolve after 10 seconds"

go-sleep 30s

verify_can_resolve "$num_test_nodes" "$KEY2" "$HASH" "new key can resolve again after republish"

#

teardown_iptb

test_done
