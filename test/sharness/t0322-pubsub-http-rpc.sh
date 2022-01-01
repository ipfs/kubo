#!/usr/bin/env bash

test_description="Test pubsub command behavior over HTTP RPC API"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon --enable-pubsub-experiment

# Require topic as multibase
# https://github.com/ipfs/go-ipfs/pull/8183
test_expect_success "/api/v0/pubsub/pub URL arg must be multibase encoded" '
  echo test > data.txt &&
  curl -s -X POST -F "data=@data.txt" "$API_ADDR/api/v0/pubsub/pub?arg=foobar" > result &&
  test_should_contain "error" result &&
  test_should_contain "URL arg must be multibase encoded" result
'

# Use URL-safe multibase
# base64 should produce error when used in URL args, base64url should be used
test_expect_success "/api/v0/pubsub/pub URL arg must be in URL-safe multibase" '
  echo test > data.txt &&
  curl -s -X POST -F "data=@data.txt" "$API_ADDR/api/v0/pubsub/pub?arg=mZm9vYmFyCg" > result &&
  test_should_contain "error" result &&
  test_should_contain "URL arg must be base64url encoded" result
'

test_kill_ipfs_daemon
test_done
