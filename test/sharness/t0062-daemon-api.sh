#!/bin/sh
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

test_init_ipfs

differentport=$((PORT_API + 1))
differentapi="/ip4/127.0.0.1/tcp/$differentport"
peerid=$(ipfs config Identity.PeerID)

test_client() {
    args="$@"
    printf $peerid >expected
    ipfs $args id -f="<id>" >actual
    test_cmp expected actual
}

test_expect_success "client should work when there is no api file and no --api is specified" '
  test_client
'

test_expect_success "client should err when there is no api file and with --api is specified" '
  test_must_fail test_client --api "$differentapi"
'

test_launch_ipfs_daemon

test_expect_success "'ipfs daemon' creates api file" '
  test -f ".ipfs/api"
'

test_expect_success "api file looks good" '
  printf "$ADDR_API" >expected &&
  test_cmp expected .ipfs/api
'

test_expect_success "client should err if client api != api file while daemon is on" '
  echo "Error: api not running" >expected &&
  test_must_fail ipfs --api "$differentapi" id 2>actual &&
  test_cmp expected actual
'

test_kill_ipfs_daemon

test_expect_success "client should err if client api != api file while daemon is off" '
  echo "Error: api not running" >expected &&
  test_must_fail ipfs --api "$differentapi" id 2>actual &&
  test_cmp expected actual
'

PORT_API=$differentport
ADDR_API=$differentapi

test_launch_ipfs_daemon --api "$ADDR_API"

test_expect_success "'ipfs daemon' api option works" '
  printf "$differentapi" >expected &&
  test_cmp expected .ipfs/api
'

test_expect_success "client should work if client api == api file, != cfg api while daemon is on" '
  test_client --api "$differentapi"
'

test_expect_success "client should read the api file while daemon is on" '
  test_client
'

test_kill_ipfs_daemon

test_expect_success "client should work if there is api file while daemon is off" '
  ipfs id
'

test_done
