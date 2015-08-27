#!/bin/sh
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "client should work when there is no api file" '
  ipfs --api "$differentapi" id
'

test_launch_ipfs_daemon

test_expect_success "'ipfs daemon' creates api file" '
  test -f ".ipfs/api"
'

differentport=$((PORT_API + 1))
differentapi="/ip4/127.0.0.1/tcp/$differentport"

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

# test_launch_ipfs_daemon '--api "$ADDR_API"'

#pasted from test_launch_ipfs_daemon because the above line doesn't work
test_expect_success "'ipfs daemon' succeeds" '
  ipfs daemon --api $ADDR_API >actual_daemon 2>daemon_err &
'

# we say the daemon is ready when the API server is ready.
test_expect_success "'ipfs daemon' is ready" '
  IPFS_PID=$! &&
  pollEndpoint -ep=/version -host=$ADDR_API -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
  test_fsh cat actual_daemon || test_fsh cat daemon_err || test_fsh cat poll_apierr || test_fsh cat poll_apiout
'
#end of "test_launch_ipfs_daemon

test_expect_success "'ipfs daemon' api option works" '
  printf "$differentapi" > expected &&
  test_cmp expected .ipfs/api
'

test_expect_success "client should work if client api == api file, != cfg api while daemon is on" '
  ipfs --api "$differentapi" id
'

test_kill_ipfs_daemon

test_expect_success "client should work if there is api file while daemon is off" '
  ipfs id
'

test_done
