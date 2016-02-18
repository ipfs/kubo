#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh


test_init_ipfs

test_launch_ipfs_daemon --unrestricted-api --disable-transport-encryption

test_expect_success "convert addresses from multiaddrs" '
'

gwyaddr=$GWAY_ADDR
apiaddr=$API_ADDR

test_expect_success 'api gateway should be unrestricted' '
  echo "hello mars :$gwyaddr :$apiaddr" >expected &&
  HASH=$(ipfs add -q expected) &&
  curl -sfo actual1 "http://$gwyaddr/ipfs/$HASH" &&
  curl -sfo actual2 "http://$apiaddr/ipfs/$HASH" &&
  test_cmp expected actual1 &&
  test_cmp expected actual2
'

# Odd. this fails here, but the inverse works on t0060-daemon.
test_expect_success 'transport should be unencrypted' '
  go-sleep 0.5s | nc localhost "$SWARM_PORT" >swarmnc &&
  test_must_fail grep -q "AES-256,AES-128" swarmnc &&
  grep -q "/multistream/1.0.0" swarmnc ||
  test_fsh cat swarmnc
'

test_kill_ipfs_daemon

test_done
