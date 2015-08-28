#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh


test_init_ipfs

test_launch_ipfs_daemon --unrestricted-api --disable-transport-encryption

gwyport=$PORT_GWAY
apiport=$PORT_API

test_expect_success 'api gateway should be unrestricted' '
  echo "hello mars :$gwyport :$apiport" >expected &&
  HASH=$(ipfs add -q expected) &&
  curl -sfo actual1 "http://127.0.0.1:$gwyport/ipfs/$HASH" &&
  curl -sfo actual2 "http://127.0.0.1:$apiport/ipfs/$HASH" &&
  test_cmp expected actual1 &&
  test_cmp expected actual2
'

# Odd. this fails here, but the inverse works on t0060-daemon.
test_expect_success 'transport should be unencrypted' '
  go-sleep 0.5s | nc localhost "$PORT_SWARM" >swarmnc &&
  test_must_fail grep -q "AES-256,AES-128" swarmnc &&
  grep -q "/ipfs/identify" swarmnc ||
  test_fsh cat swarmnc
'

test_kill_ipfs_daemon

test_done
