#!/usr/bin/env bash
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh


test_init_ipfs

test_launch_ipfs_daemon --disable-transport-encryption

gwyaddr=$GWAY_ADDR
apiaddr=$API_ADDR

# Odd. this fails here, but the inverse works on t0060-daemon.
test_expect_success SOCAT 'transport should be unencrypted ( needs socat )' '
  socat - tcp:localhost:$SWARM_PORT,connect-timeout=1 > swarmnc < ../t0060-data/mss-ls &&
  grep -q "/plaintext" swarmnc ||
  test_fsh cat swarmnc
'

test_kill_ipfs_daemon

test_launch_ipfs_daemon_without_network

gwyaddr=$GWAY_ADDR
apiaddr=$API_ADDR

test_expect_success 'gateway should work in offline mode' '
  echo "hello mars :$gwyaddr :$apiaddr" >expected &&
  HASH=$(ipfs add -q expected) &&
  curl -sfo actual1 "http://$gwyaddr/ipfs/$HASH" &&
  test_cmp expected actual1
'

test_kill_ipfs_daemon

test_expect_success 'daemon should not start with bad dht opt' '
  test_must_fail ipfs daemon --routing=fdsfdsfds > daemon_output 2>&1
'

test_expect_success 'output contains info about dht option' '
  grep "unrecognized routing option:" daemon_output ||
  test_fsh cat daemon_output
'

test_expect_success 'daemon should not start with supernode dht opt' '
  test_must_fail ipfs daemon --routing=supernode > daemon_output2 2>&1
'

test_expect_success 'output contains info about supernode dht option' '
  grep "supernode routing was never fully implemented" daemon_output2 ||
  test_fsh cat daemon_output2
'

test_done
