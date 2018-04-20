#!/usr/bin/env bash

test_description="Test non-standard datastores"

. lib/test-lib.sh

test_expect_success "'ipfs init --profile=badgerds' succeeds" '
  BITS="1024" &&
  ipfs init --bits="$BITS" --profile=badgerds
'

test_expect_success "'ipfs pin ls' works" '
  ipfs pin ls | wc -l | grep 9
'

test_done
