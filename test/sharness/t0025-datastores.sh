#!/usr/bin/env bash

test_description="Test non-standard datastores"

. lib/test-lib.sh

test_expect_success "'ipfs init --profile=badgerds' succeeds" '
  ipfs init --profile=badgerds
'

test_expect_success "'ipfs pin ls' works" '
  ipfs pin ls | wc -l | grep 9
'

test_expect_success "cleanup repo" '
  rm -rf "$IPFS_PATH"
'

test_expect_success "'ipfs init --profile=badger2ds' succeeds" '
  ipfs init --profile=badger2ds
'

test_expect_success "'ipfs pin ls' works" '
  ipfs pin ls | wc -l | grep 9
'

test_done
